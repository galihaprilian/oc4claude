package main

import (
	"os"
	"path/filepath"
	"strings"
)

func DetectWSL() bool {
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	content := strings.ToLower(string(data))
	return strings.Contains(content, "wsl") || strings.Contains(content, "microsoft")
}

func GetAutostartPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "autostart", "oc4claude.desktop")
}

func Install() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	autostartDir := filepath.Join(home, ".config", "autostart")
	if err := os.MkdirAll(autostartDir, 0755); err != nil {
		return err
	}

	execPath, err := os.Executable()
	if err != nil {
		return err
	}

	desktopEntry := "[Desktop Entry]\n" +
		"Type=Application\n" +
		"Name=oc4claude\n" +
		"Exec=" + execPath + " start\n" +
		"Hidden=false\n" +
		"X-GNOME-Autostart-enabled=true\n"

	desktopPath := filepath.Join(autostartDir, "oc4claude.desktop")
	return os.WriteFile(desktopPath, []byte(desktopEntry), 0644)
}

func Uninstall() error {
	desktopPath := GetAutostartPath()
	if desktopPath == "" {
		return nil
	}
	err := os.Remove(desktopPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}