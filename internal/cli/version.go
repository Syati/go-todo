package cli

import "runtime/debug"

func buildVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "development"
	}
	revision := ""
	modified := false
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.modified":
			modified = setting.Value == "true"
		}
	}
	if revision == "" {
		if info.Main.Version != "" && info.Main.Version != "(devel)" {
			return info.Main.Version
		}
		return "development"
	}
	if modified {
		return revision + "-dirty"
	}
	return revision
}
