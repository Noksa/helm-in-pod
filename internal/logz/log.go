package logz

import (
	"fmt"

	"github.com/fatih/color"
)

func LogHost() string {
	return fmt.Sprintf("[%v]", color.CyanString("Host"))
}

func LogPod() string {
	return fmt.Sprintf("[%v]", color.MagentaString("Pod"))
}
