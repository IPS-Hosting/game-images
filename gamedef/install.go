package gamedef

import (
	"fmt"
	"path"
	"strconv"
	"strings"
)

// DefaultInstallerImage runs steamcmd, script and download stages.
const DefaultInstallerImage = "ipshosting/installer:v1"

// SteamcmdRunscriptPath is where the engine places the generated steamcmd runscript inside
// the installer container. Credentials and branch passwords go into the runscript, never
// into argv or the container environment.
const SteamcmdRunscriptPath = "/tmp/ips-steamcmd.txt"

// InstallOptions control the install rendering.
type InstallOptions struct {
	// Validate requests a full file validation (steamcmd validate, download re-check).
	Validate bool
	// InstallerImage overrides DefaultInstallerImage.
	InstallerImage string
}

// InstallPlan is the rendered install/update/validate procedure.
type InstallPlan struct {
	Stages      []PlanStage  `json:"stages"`
	PostInstall []PlanHookOp `json:"postInstall,omitempty"`
}

// PlanStage is one rendered install stage, run as its own container with the data
// directory mounted at the definition's dataPath.
type PlanStage struct {
	Kind       string            `json:"kind"`
	Image      string            `json:"image"`
	Command    []string          `json:"command,omitempty"`
	Env        map[string]string `json:"env"`
	WorkingDir string            `json:"workingDir"`
	// Steamcmd is set for steamcmd stages; the engine writes its runscript (with the
	// login line) to SteamcmdRunscriptPath before starting the container.
	Steamcmd *SteamcmdStage `json:"steamcmd,omitempty"`
	// Download is set for download stages.
	Download *DownloadStage `json:"download,omitempty"`
}

// SteamcmdStage is a rendered steamcmd stage.
type SteamcmdStage struct {
	AppID          int      `json:"appId"`
	Dir            string   `json:"dir"`
	Login          string   `json:"login"`
	Platform       string   `json:"platform,omitempty"`
	Branch         string   `json:"branch,omitempty"`
	BranchPassword string   `json:"branchPassword,omitempty"`
	Validate       bool     `json:"validate"`
	ExtraArgs      []string `json:"extraArgs,omitempty"`
}

// DownloadStage is a rendered download stage.
type DownloadStage struct {
	URL             string `json:"url"`
	Checksum        string `json:"checksum,omitempty"`
	Dest            string `json:"dest"`
	Format          string `json:"format"`
	StripComponents int    `json:"stripComponents"`
}

// Runscript returns the steamcmd runscript. username and password are only used for
// login "host"; anonymous stages ignore them.
func (s *SteamcmdStage) Runscript(username string, password string) (string, error) {
	if err := checkSteamTokens(append([]string{s.Dir, s.Branch, s.BranchPassword, username, password}, s.ExtraArgs...)...); err != nil {
		return "", err
	}
	var lines []string
	lines = append(lines, "@ShutdownOnFailedCommand 1", "@NoPromptForPassword 1")
	if s.Platform != "" {
		lines = append(lines, "@sSteamCmdForcePlatformType "+s.Platform)
	}
	lines = append(lines, "force_install_dir "+quoteSteam(s.Dir))
	if s.Login == "host" {
		if username == "" {
			return "", fmt.Errorf("steamcmd stage for app %d needs host steam credentials", s.AppID)
		}
		lines = append(lines, "login "+quoteSteam(username)+" "+quoteSteam(password))
	} else {
		lines = append(lines, "login anonymous")
	}
	update := []string{"app_update", strconv.Itoa(s.AppID)}
	if s.Branch != "" {
		update = append(update, "-beta", quoteSteam(s.Branch))
		if s.BranchPassword != "" {
			update = append(update, "-betapassword", quoteSteam(s.BranchPassword))
		}
	}
	update = append(update, s.ExtraArgs...)
	if s.Validate {
		update = append(update, "validate")
	}
	lines = append(lines, strings.Join(update, " "), "quit")
	return strings.Join(lines, "\n") + "\n", nil
}

// quoteSteam quotes a runscript token. Tokens are checked by checkSteamTokens first:
// steamcmd has no escape for embedded quotes.
func quoteSteam(token string) string {
	return `"` + token + `"`
}

// checkSteamTokens rejects tokens that cannot be quoted safely in a runscript.
func checkSteamTokens(tokens ...string) error {
	for _, token := range tokens {
		if strings.ContainsAny(token, "\"\r\n\x00") {
			return fmt.Errorf("steamcmd values must not contain quotes or line breaks")
		}
	}
	return nil
}

// RenderInstall renders the install plan of one game server.
func RenderInstall(d *Definition, values Values, options InstallOptions) (*InstallPlan, error) {
	r := newRenderer(d, values)
	installer := options.InstallerImage
	if installer == "" {
		installer = DefaultInstallerImage
	}
	plan := &InstallPlan{}

	for i, stage := range d.Install {
		at := fmt.Sprintf("/install/%d", i)
		include, err := r.when(at+"/when", stage.When)
		if err != nil {
			return nil, err
		}
		if !include {
			continue
		}
		rendered := PlanStage{Kind: stage.Kind, Image: installer, Env: baseEnv(d, values), WorkingDir: d.Runtime.DataPath}
		if err := r.env(at+"/env", stage.Env, rendered.Env); err != nil {
			return nil, err
		}

		switch stage.Kind {
		case StageSteamcmd:
			dir := d.Runtime.DataPath
			if stage.Dir != "" {
				rel, err := r.relPath(at+"/dir", stage.Dir)
				if err != nil {
					return nil, err
				}
				dir = path.Join(d.Runtime.DataPath, rel)
			}
			branch, err := r.scalar(at+"/branch", stage.Branch)
			if err != nil {
				return nil, err
			}
			branchPassword, err := r.text(at+"/branchPassword", stage.BranchPassword)
			if err != nil {
				return nil, err
			}
			if strings.ContainsAny(branch+branchPassword, "\"\r\n") {
				return nil, fmt.Errorf("%s: branch and branch password must not contain quotes or line breaks", at)
			}
			rendered.Steamcmd = &SteamcmdStage{
				AppID: stage.AppID, Dir: dir, Login: stage.Login, Platform: stage.Platform,
				Branch: branch, BranchPassword: branchPassword, Validate: options.Validate, ExtraArgs: stage.ExtraArgs,
			}
			rendered.Command = []string{"steamcmd", "+runscript", SteamcmdRunscriptPath}

		case StageDownload:
			url, err := r.scalar(at+"/url", stage.URL)
			if err != nil {
				return nil, err
			}
			if !strings.HasPrefix(url, "https://") {
				return nil, fmt.Errorf("%s/url: only https downloads are allowed", at)
			}
			dest := "."
			if stage.Dest != "" {
				if dest, err = r.relPath(at+"/dest", stage.Dest); err != nil {
					return nil, err
				}
			}
			download := &DownloadStage{URL: url, Checksum: stage.Checksum, Dest: dest, Format: "none"}
			if stage.Extract != nil {
				if stage.Extract.Format != "" {
					download.Format = stage.Extract.Format
				}
				download.StripComponents = stage.Extract.StripComponents
			}
			rendered.Download = download

		case StageScript:
			if stage.Image != "" {
				image, err := r.scalar(at+"/image", stage.Image)
				if err != nil {
					return nil, err
				}
				if err := CheckImage(image); err != nil {
					return nil, fmt.Errorf("%s/image: %w", at, err)
				}
				rendered.Image = image
			}
			rendered.Command = []string{"/bin/sh", "-c", stage.Shell}

		case StageContainer:
			image, err := r.scalar(at+"/image", stage.Image)
			if err != nil {
				return nil, err
			}
			if err := CheckImage(image); err != nil {
				return nil, fmt.Errorf("%s/image: %w", at, err)
			}
			rendered.Image = image
			for j, arg := range stage.Args {
				value, err := r.text(fmt.Sprintf("%s/args/%d", at, j), arg)
				if err != nil {
					return nil, err
				}
				if strings.ContainsRune(value, 0) {
					return nil, fmt.Errorf("%s/args/%d: argument contains a NUL byte", at, j)
				}
				rendered.Command = append(rendered.Command, value)
			}

		default:
			return nil, fmt.Errorf("%s: unsupported install stage %q", at, stage.Kind)
		}
		plan.Stages = append(plan.Stages, rendered)
	}

	var err error
	plan.PostInstall, err = r.hooks("/hooks/postInstall", d.Hooks.PostInstall)
	if err != nil {
		return nil, err
	}
	return plan, nil
}
