package hipembedded

import (
	_ "embed"
)

//go:embed script.sh
var shScript string

func GetShScript() string {
	return shScript
}
