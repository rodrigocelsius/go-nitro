package main

// frameworkVersion is the go-nitro release that generated projects require.
// scripts/release.sh refuses to tag a release while it differs from the
// version being released, so a new CLI never scaffolds an older framework.
const frameworkVersion = "v2.2.0"
