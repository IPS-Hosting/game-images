package gamedef

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode/utf8"
)

// xmlNode is a minimal XML tree that keeps comments, processing instructions, directives,
// whitespace and self-closing tags, so unmanaged content survives a rewrite.
type xmlNode struct {
	token       xml.Token // nil for elements
	name        string
	attrs       []xml.Attr
	children    []*xmlNode
	selfClosing bool
	closeSpace  string // whitespace before "/>" of a self-closing tag
}

func (n *xmlNode) isElement() bool { return n.token == nil }

// xmlSegment is one element step of an xml key: a name and an optional attribute predicate.
type xmlSegment struct {
	name, attr, value string
}

var xmlSegmentPattern = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_:-]*)(?:\[([A-Za-z_][A-Za-z0-9_:-]*)=([^\]\r\n\x00]*)\])?$`)

// parseXMLKey splits "A.B[attr=value].C" or "A.B@attr" into element steps and the target
// attribute. Predicate values may contain "." and "@" but not "]".
func parseXMLKey(key string) ([]xmlSegment, string, error) {
	var parts []string
	attr, start, inPredicate, hasAttr := "", 0, false, false
	for i := 0; i < len(key) && !hasAttr; i++ {
		switch {
		case key[i] == '[' && !inPredicate:
			inPredicate = true
		case key[i] == ']' && inPredicate:
			inPredicate = false
		case (key[i] == '.' || key[i] == '@') && !inPredicate:
			parts = append(parts, key[start:i])
			start = i + 1
			if key[i] == '@' {
				attr, hasAttr = key[i+1:], true
			}
		}
	}
	if !hasAttr {
		parts = append(parts, key[start:])
	}
	if hasAttr && !xmlNamePattern.MatchString(attr) {
		return nil, "", fmt.Errorf("invalid attribute name in key %q", key)
	}
	segments := make([]xmlSegment, 0, len(parts))
	for _, part := range parts {
		match := xmlSegmentPattern.FindStringSubmatch(part)
		if match == nil {
			return nil, "", fmt.Errorf("invalid element path in key %q (A.B.C, A.B[attr=value] or A.B@attr; predicate values must not contain ])", key)
		}
		segments = append(segments, xmlSegment{name: match[1], attr: match[2], value: match[3]})
	}
	return segments, attr, nil
}

func xmlName(name xml.Name) string {
	if name.Space != "" {
		return name.Space + ":" + name.Local
	}
	return name.Local
}

// parseXML reads a document into a synthetic root node whose children are the top-level
// tokens.
func parseXML(data []byte) (*xmlNode, error) {
	document := &xmlNode{}
	stack := []*xmlNode{document}
	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.Strict = true
	for {
		offset := decoder.InputOffset()
		token, err := decoder.RawToken()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		parent := stack[len(stack)-1]
		switch t := token.(type) {
		case xml.StartElement:
			end := decoder.InputOffset()
			raw := data[offset:end]
			node := &xmlNode{name: xmlName(t.Name), attrs: t.Attr, selfClosing: bytes.HasSuffix(raw, []byte("/>"))}
			if node.selfClosing {
				tag := raw[:len(raw)-2]
				node.closeSpace = string(tag[len(bytes.TrimRight(tag, " \t\r\n")):])
			}
			parent.children = append(parent.children, node)
			stack = append(stack, node)
		case xml.EndElement:
			if len(stack) < 2 || parent.name != xmlName(t.Name) {
				return nil, fmt.Errorf("unexpected end element </%s>", xmlName(t.Name))
			}
			stack = stack[:len(stack)-1]
		default:
			parent.children = append(parent.children, &xmlNode{token: xml.CopyToken(token)})
		}
	}
	if len(stack) != 1 {
		return nil, errors.New("unclosed element")
	}
	return document, nil
}

// child returns the first child element matching the segment (name and predicate).
func (n *xmlNode) child(segment xmlSegment) *xmlNode {
	for _, child := range n.children {
		if child.isElement() && child.name == segment.name && (segment.attr == "" || child.hasAttr(segment.attr, segment.value)) {
			return child
		}
	}
	return nil
}

// addChild appends a new element (with the predicate attribute set), copying the indentation of existing children when the
// parent is indented.
func (n *xmlNode) addChild(segment xmlSegment) *xmlNode {
	child := &xmlNode{name: segment.name, selfClosing: segment.attr != ""}
	if segment.attr != "" {
		child.setAttr(segment.attr, segment.value)
	}
	n.selfClosing = false
	var indent, closing *xmlNode
	if len(n.children) >= 2 {
		if text, ok := n.children[len(n.children)-1].token.(xml.CharData); ok && len(bytes.TrimSpace(text)) == 0 {
			closing = n.children[len(n.children)-1]
			for i := len(n.children) - 2; i >= 0; i-- {
				if t, ok := n.children[i].token.(xml.CharData); ok && len(bytes.TrimSpace(t)) == 0 && i+1 < len(n.children) && n.children[i+1].isElement() {
					indent = &xmlNode{token: xml.CopyToken(t)}
					break
				}
			}
		}
	}
	if closing != nil && indent != nil {
		n.children = append(n.children[:len(n.children)-1], indent, child, closing)
	} else {
		n.children = append(n.children, child)
	}
	return child
}

// applyXML implements format xml (SPEC §5.4).
func applyXML(existing []byte, values map[string]string) ([]byte, error) {
	document := &xmlNode{}
	if len(bytes.TrimSpace(existing)) > 0 {
		var err error
		if document, err = parseXML(existing); err != nil {
			return nil, fmt.Errorf("existing file is not valid XML: %w", err)
		}
	}
	for _, key := range sortedKeys(values) {
		segments, attr, err := parseXMLKey(key)
		if err != nil {
			return nil, err
		}
		if err := checkXMLText(values[key]); err != nil {
			return nil, fmt.Errorf("key %q: %w", key, err)
		}
		node := document.child(segments[0])
		if node == nil {
			for _, child := range document.children {
				if child.isElement() {
					return nil, fmt.Errorf("key %q: the root element <%s> does not match", key, child.name)
				}
			}
			node = document.addChild(segments[0])
		}
		for _, segment := range segments[1:] {
			next := node.child(segment)
			if next == nil {
				next = node.addChild(segment)
			}
			node = next
		}
		if attr != "" {
			node.setAttr(attr, values[key])
			continue
		}
		for _, child := range node.children {
			if child.isElement() {
				return nil, fmt.Errorf("key %q: element has child elements", key)
			}
		}
		node.selfClosing = false
		node.children = []*xmlNode{{token: xml.CharData(values[key])}}
	}
	var out bytes.Buffer
	for _, child := range document.children {
		writeXML(&out, child)
	}
	if out.Len() > 0 && !bytes.HasSuffix(out.Bytes(), []byte("\n")) {
		out.WriteByte('\n')
	}
	result := out.Bytes()
	// An XML declaration must be the very first thing in the file (after an optional BOM);
	// strict parsers (.NET) reject whitespace before it, which some shipped templates have.
	bom := []byte("\xEF\xBB\xBF")
	body, hadBOM := bytes.CutPrefix(result, bom)
	if trimmed := bytes.TrimLeft(body, " \t\r\n"); bytes.HasPrefix(trimmed, []byte("<?xml")) {
		result = trimmed
		if hadBOM {
			result = append(append([]byte{}, bom...), trimmed...)
		}
	}
	// The decoder normalizes line endings; restore CRLF files.
	if bytes.Contains(existing, []byte("\r\n")) {
		return bytes.ReplaceAll(bytes.ReplaceAll(result, []byte("\r\n"), []byte("\n")), []byte("\n"), []byte("\r\n")), nil
	}
	return result, nil
}

func (n *xmlNode) hasAttr(name string, value string) bool {
	for _, attr := range n.attrs {
		if xmlName(attr.Name) == name && attr.Value == value {
			return true
		}
	}
	return false
}

func (n *xmlNode) setAttr(name string, value string) {
	for i := range n.attrs {
		if xmlName(n.attrs[i].Name) == name {
			n.attrs[i].Value = value
			return
		}
	}
	space, local, found := strings.Cut(name, ":")
	if !found {
		space, local = "", name
	}
	n.attrs = append(n.attrs, xml.Attr{Name: xml.Name{Space: space, Local: local}, Value: value})
}

func writeXML(out *bytes.Buffer, n *xmlNode) {
	switch t := n.token.(type) {
	case nil:
		out.WriteString("<" + n.name)
		for _, attr := range n.attrs {
			out.WriteString(" " + xmlName(attr.Name) + `="`)
			out.WriteString(xmlAttrEscaper.Replace(attr.Value))
			out.WriteByte('"')
		}
		if n.selfClosing && len(n.children) == 0 {
			out.WriteString(n.closeSpace + "/>")
			return
		}
		out.WriteByte('>')
		for _, child := range n.children {
			writeXML(out, child)
		}
		out.WriteString("</" + n.name + ">")
	case xml.CharData:
		// xml.EscapeText would also escape newlines and destroy the indentation.
		out.WriteString(xmlTextEscaper.Replace(string(t)))
	case xml.Comment:
		out.WriteString("<!--" + string(t) + "-->")
	case xml.ProcInst:
		out.WriteString("<?" + t.Target)
		if len(t.Inst) > 0 {
			out.WriteString(" " + string(t.Inst))
		}
		out.WriteString("?>")
	case xml.Directive:
		out.WriteString("<!" + string(t) + ">")
	}
}

var (
	xmlTextEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	xmlAttrEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "\t", "&#x9;", "\n", "&#xA;", "\r", "&#xD;")
)

// checkXMLText rejects values that cannot appear in an XML 1.0 document: invalid UTF-8
// and characters outside the XML Char production.
func checkXMLText(value string) error {
	if !utf8.ValidString(value) {
		return fmt.Errorf("value is not valid UTF-8")
	}
	for _, r := range value {
		if !(r == 0x9 || r == 0xA || r == 0xD || (r >= 0x20 && r <= 0xD7FF) || (r >= 0xE000 && r <= 0xFFFD) || (r >= 0x10000 && r <= 0x10FFFF)) {
			return fmt.Errorf("value contains the character %U, which XML 1.0 does not allow", r)
		}
	}
	return nil
}
