package main

import (
	"fmt"
	"os"
	"runtime"
)

func main() {
	fmt.Println("==================================================")
	fmt.Println("🎉 Hello World from containerized macOS via Podman!")
	fmt.Printf("   OS:       %s\n", runtime.GOOS)
	fmt.Printf("   Arch:     %s\n", runtime.GOARCH)
	fmt.Printf("   GoVer:    %s\n", runtime.Version())
	hostname, _ := os.Hostname()
	fmt.Printf("   Hostname: %s\n", hostname)
	fmt.Println("==================================================")
}
