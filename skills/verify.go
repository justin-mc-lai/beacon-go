// Skill install verification (skills-distribution): compare each
// host's installed beacon-go* set with the 1+6 surface this binary
// embeds. v1's `beacon-*` names are outside this namespace and are
// never reported as extra, deleted, or overwritten.
package skills

import "sort"

// HostVerification is one host's drift report.
type HostVerification struct {
	Host    string   `json:"host"`
	Dir     string   `json:"dir"`
	Exists  bool     `json:"exists"`
	Missing []string `json:"missing,omitempty"`
	Extra   []string `json:"extra,omitempty"`
	OK      bool     `json:"ok"`
}

// ExpectedInstallNames is the router plus every 1+6 skill directory,
// namespaced. It is the one expected set for install and verify.
func ExpectedInstallNames() []string {
	names := []string{InstallPrefix}
	for _, s := range List() {
		names = append(names, InstallDirName(s.ID))
	}
	sort.Strings(names)
	return names
}

// VerifyHost reports what one host is missing and what it carries
// beyond the expected set. A host that does not exist is not an
// error: it is simply not installed.
func VerifyHost(h Host) HostVerification {
	expected := ExpectedInstallNames()
	expectedSet := make(map[string]bool, len(expected))
	for _, name := range expected {
		expectedSet[name] = true
	}
	installed := make(map[string]bool, len(h.Installed))
	for _, name := range h.Installed {
		installed[name] = true
	}
	v := HostVerification{Host: h.Name, Dir: h.Dir, Exists: h.Exists}
	for _, name := range expected {
		if !installed[name] {
			v.Missing = append(v.Missing, name)
		}
	}
	for _, name := range h.Installed {
		if !expectedSet[name] {
			v.Extra = append(v.Extra, name)
		}
	}
	sort.Strings(v.Missing)
	sort.Strings(v.Extra)
	v.OK = len(v.Missing) == 0 && len(v.Extra) == 0
	return v
}

// VerifyAll verifies the whole host table.
func VerifyAll(home string) ([]HostVerification, error) {
	hosts, err := HostDirs(home)
	if err != nil {
		return nil, err
	}
	out := make([]HostVerification, 0, len(hosts))
	for _, h := range hosts {
		out = append(out, VerifyHost(h))
	}
	return out, nil
}
