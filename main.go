package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/getlantern/systray"
)

const baseDir = `C:\Battlestate Games\EFT\Logs`

func main() {
	systray.Run(onReady, nil)
}

type IPResponse struct {
	IP          string `json:"ip"`
	CountryName string `json:"country_name"`
}

func getCountryNameForIP(ip string) (string, error) {
	resp, err := http.Get("https://api.iplocation.net/?ip=" + ip)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var ipResp IPResponse
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&ipResp); err != nil {
		return "", err
	}

	return ipResp.CountryName, nil
}

func onReady() {
	var latestIP, lastUpdated string

	mShowIP := systray.AddMenuItem("Show IP & Country", "Display the latest IP and country")
	mQuit := systray.AddMenuItem("Quit", "Quit the app")

	checkIPAndUpdateTray(&latestIP, &lastUpdated)

	go func() {
		for {
			select {
			case <-mQuit.ClickedCh:
				systray.Quit()
				return
			case <-mShowIP.ClickedCh:
				if latestIP != "" {
					systray.SetTitle(fmt.Sprintf("IP: %s | Last Updated: %s", latestIP, lastUpdated))
					fmt.Println("IP:", latestIP, "| Last Updated:", lastUpdated)
				} else {
					systray.SetTitle("IP not yet determined")
				}
			case <-time.After(1 * time.Minute):
				checkIPAndUpdateTray(&latestIP, &lastUpdated)
			}
		}
	}()
}

func checkIPAndUpdateTray(latestIP *string, lastUpdated *string) {
	logFilePath, err := getLatestLogFilePath(baseDir)
	if err != nil {
		fmt.Println("Error retrieving log file:", err)
		return
	}

	ip, err := getIPFromMatchingLine(logFilePath)
	if err != nil {
		fmt.Println("Error extracting IP:", err)
		return
	}

	countryName, err := getCountryNameForIP(ip)
	if err != nil {
		fmt.Println("Error getting country name for IP:", err)
		return
	}

	*latestIP = ip
	*lastUpdated = time.Now().Format("15:04")

	systray.SetTooltip(fmt.Sprintf("IP: %s | Country: %s | Last Updated: %s", *latestIP, countryName, *lastUpdated))
}

func getLatestLogFilePath(dirPath string) (string, error) {
	dir, err := os.Open(dirPath)
	if err != nil {
		return "", err
	}
	defer dir.Close()

	files, err := dir.Readdir(-1) // -1 means to return all entries
	if err != nil {
		return "", err
	}

	var latestDir os.FileInfo
	for _, file := range files {
		if file.IsDir() {
			if latestDir == nil || file.ModTime().After(latestDir.ModTime()) {
				latestDir = file
			}
		}
	}

	if latestDir == nil {
		return "", fmt.Errorf("no directories found in %s", dirPath)
	}

	fmt.Println("Latest directory:", latestDir.Name()) // Debugging line

	// Stripping "log_" prefix for the log file name and appending " application.log"
	logFileName := fmt.Sprintf("%s application.log", strings.TrimPrefix(latestDir.Name(), "log_"))
	logFilePath := filepath.Join(dirPath, latestDir.Name(), logFileName)

	fmt.Println("Trying to open log file at path:", logFilePath) // Debugging line

	return logFilePath, nil
}

func getIPFromMatchingLine(logFilePath string) (string, error) {
	file, err := os.Open(logFilePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	var matchingLines []string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "RaidMode: Online,") {
			matchingLines = append(matchingLines, line)
		}
	}

	if len(matchingLines) == 0 {
		return "", fmt.Errorf("no matching line found in log file")
	}

	latestLine := matchingLines[len(matchingLines)-1]
	re := regexp.MustCompile(`Ip: ([\d\.]+),`)
	matches := re.FindStringSubmatch(latestLine)

	if len(matches) < 2 {
		return "", fmt.Errorf("could not extract IP from line")
	}

	return matches[1], nil
}
