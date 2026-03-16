package config

var (
	debug  bool
	dryRun bool
)

func SetDebug(v bool) { debug = v }
func Debug() bool     { return debug }

func SetDryRun(v bool) { dryRun = v }
func DryRun() bool     { return dryRun }
