package vm

// FreeSubnets lists the job subnets free of the given `netstat` routes.
func FreeSubnets(routes string, n int) []string {
	var out []string
	for _, p := range freeSubnets(routes, n) {
		out = append(out, p.String())
	}
	return out
}

// NPMInstallScriptForTest is the VM setup step that installs node_modules.
func NPMInstallScriptForTest() string { return npmInstallScript }

// NixScriptForTest is the VM setup step that enters a flake's dev shell, with the
// Nix root and seed moved to directories the test owns.
func NixScriptForTest(s *ContainerSandbox, root, seed, proxyURL string) string {
	return s.nixScriptAt(root, seed, proxyURL)
}
