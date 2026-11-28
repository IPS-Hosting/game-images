package gamedef

import (
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"
)

// DefaultInstallerImage runs steamcmd, script and download stages.
const DefaultInstallerImage = "ipshosting/installer:v1"

// SteamcmdRunscriptPath is where the engine places the generated steamcmd runscript inside
// the installer container. Credentials and branch passwords go into the runscript, never
// into argv or the container environment.
const SteamcmdRunscriptPath = "/tmp/ips-steamcmd.txt"

// SteamHomePath is HOME of steamcmd stages: a private per-stage directory outside the data
// directory (Steam config and token cache), mounted by the engine and deleted after the stage.
const SteamHomePath = EnginePath + "/steam-home"

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
	URL string `json:"url"`
	// Checksum is always set (sha256:<hex>); engines refuse a download stage without it.
	Checksum        string `json:"checksum"`
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
	r, err := newRenderer(d, values)
	if err != nil {
		return nil, err
	}
	installer := options.InstallerImage
	if installer == "" {
		installer = DefaultInstallerImage
	}
	plan := &InstallPlan{}
	var steamSDK []PlanHookOp

	for i, stage := range d.Install {
		if !r.when(stage.When) {
			continue
		}
		at := fmt.Sprintf("/install/%d", i)
		rendered := PlanStage{Kind: stage.Kind, Image: installer, Env: baseEnv(d, values), WorkingDir: d.Run.DataPath}
		if err := r.env(at+"/env", stage.Env, rendered.Env); err != nil {
			return nil, err
		}

		switch stage.Kind {
		case StageSteamcmd:
			dir := "."
			if stage.Dir != "" {
				dir = path.Clean(stage.Dir)
			}
			branch, password, err := r.branch(at, stage)
			if err != nil {
				return nil, err
			}
			rendered.Env["HOME"] = SteamHomePath
			rendered.Steamcmd = &SteamcmdStage{
				AppID: stage.AppID, Dir: path.Join(d.Run.DataPath, dir), Login: stage.Login, Platform: stage.Platform,
				Branch: branch, BranchPassword: password, Validate: options.Validate, ExtraArgs: stage.ExtraArgs,
			}
			rendered.Command = []string{"steamcmd", "+runscript", SteamcmdRunscriptPath}
			if stage.SteamSDK {
				steamSDK = append(steamSDK, steamSDKHooks(dir)...)
			}

		case StageDownload:
			dest := "."
			if stage.Dest != "" {
				dest = path.Clean(stage.Dest)
			}
			download := &DownloadStage{URL: stage.URL, Checksum: stage.Checksum, Dest: dest, Format: "none"}
			if stage.Extract != nil {
				if stage.Extract.Format != "" {
					download.Format = stage.Extract.Format
				}
				download.StripComponents = stage.Extract.StripComponents
			}
			rendered.Download = download

		case StageScript:
			if stage.Image != "" {
				rendered.Image = stage.Image
			}
			rendered.Command = []string{"/bin/sh", "-c", stage.Shell}

		default:
			return nil, fmt.Errorf("%s: unsupported install stage %q", at, stage.Kind)
		}
		plan.Stages = append(plan.Stages, rendered)
	}

	hooks, err := r.hooks("/hooks/postInstall", d.Hooks.PostInstall)
	if err != nil {
		return nil, err
	}
	plan.PostInstall = append(steamSDK, hooks...)
	return plan, nil
}

// branchPattern is the syntax of Steam beta branch names.
var branchPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

// branch resolves the beta branch and its password of a steamcmd stage.
func (r *renderer) branch(at string, stage InstallStage) (string, string, error) {
	var branch, password string
	if stage.Beta != nil {
		branch = strings.TrimSpace(r.settings[stage.Beta.BranchSetting])
		if strings.EqualFold(branch, "public") {
			branch = ""
		}
		if branch != "" && stage.Beta.PasswordSetting != "" {
			password = r.settings[stage.Beta.PasswordSetting]
		}
	} else if stage.Branch != "" {
		rendered, err := r.text(at+"/branch", stage.Branch)
		if err != nil {
			return "", "", err
		}
		branch = strings.TrimSpace(rendered)
	}
	if branch != "" && !branchPattern.MatchString(branch) {
		return "", "", fmt.Errorf("%s: invalid beta branch %q (letters, digits, . _ -, at most 64)", at, branch)
	}
	if err := checkSteamTokens(password); err != nil {
		return "", "", fmt.Errorf("%s: %w", at, err)
	}
	return branch, password, nil
}

// steamSDKHooks links the Steam client libraries of a steamcmd install directory to where
// SteamAPI_Init looks for them (~/.steam/sdk64 and sdk32). Steam app directories (the Steamworks SDK
// redistributable 1007 and dedicated servers) keep the 64-bit library in linux64/ and the 32-bit one at the top
// level; linux32/ exists only in the steamcmd directory itself. Links follow updates of the install directory; a
// missing 32-bit library leaves a harmless dangling link.
func steamSDKHooks(dir string) []PlanHookOp {
	var ops []PlanHookOp
	for _, lib := range []struct{ sdk, target string }{
		{".steam/sdk64", path.Join(dir, "linux64", "steamclient.so")},
		{".steam/sdk32", path.Join(dir, "steamclient.so")},
	} {
		ops = append(ops,
			PlanHookOp{EnsureDir: &EnsureDir{Path: lib.sdk}},
			PlanHookOp{Symlink: &Symlink{Target: lib.target, Link: lib.sdk + "/steamclient.so"}},
		)
	}
	return ops
}
