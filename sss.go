package main

import (
	"bufio"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const telegramBotToken = "7872022641:AAH0gTzmmz3PTwAuLATaamxiznQcewn9y0Q"
const telegramChatID = "1568047254"

func checkJupyterLab(ip string) bool {
	client := &http.Client{
		Timeout: 7 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	baseURL := fmt.Sprintf("http://%s:8888", ip)

	testPaths := []string{
		"",
		"/lab",
		"/tree",
		"/api",
		"/api/status",
	}

	for _, path := range testPaths {
		url := baseURL + path

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			continue
		}

		req.Header.Set("User-Agent", "Mozilla/5.0")
		req.Header.Set("Accept", "application/json,text/html")

		resp, err := client.Do(req)
		if err != nil {
			continue
		}

		bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 32768))
		resp.Body.Close()

		if err != nil {
			continue
		}

		body := string(bodyBytes)
		lowerBody := strings.ToLower(body)

		jupyterIndicators := []string{
			"jupyter-config-data",
			"jupyter lab",
			"jupyterlab",
			"ipython-main",
			"notebookapp",
			"base_url",
			"kernelspecs",
			"terminals",
			"/static/lab/",
			"jupyter_core",
		}

		antiIndicators := []string{
			"fastpanel",
			"cpanel",
			"plesk",
			"webmin",
			"adminer",
			"phpmyadmin",
			"cockpit",
			"vesta",
		}

		for _, anti := range antiIndicators {
			if strings.Contains(lowerBody, anti) {
				return false
			}
		}

		if resp.Header.Get("Server") == "TornadoServer/6.1" ||
			resp.Header.Get("Server") == "TornadoServer/6.0" ||
			resp.Header.Get("Server") == "TornadoServer/5.0" {
			for _, ind := range jupyterIndicators {
				if strings.Contains(lowerBody, ind) {
					return true
				}
			}
		}

		if strings.Contains(lowerBody, "jupyter-config-data") {
			return true
		}

		if path == "/api/status" || path == "/api" {
			if strings.Contains(lowerBody, "version") &&
				(strings.Contains(lowerBody, "jupyter") ||
					strings.Contains(lowerBody, "ipython")) {
				return true
			}
		}

		if resp.StatusCode == 200 {
			jupyterCount := 0
			for _, ind := range jupyterIndicators {
				if strings.Contains(lowerBody, ind) {
					jupyterCount++
				}
			}
			if jupyterCount >= 2 {
				return true
			}
		}
	}

	return false
}

func sendToTelegram(ip string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", telegramBotToken)
	message := fmt.Sprintf("Found JupyterLab: http://%s:8888", ip)

	payload := map[string]string{
		"chat_id": telegramChatID,
		"text":    message,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := http.Post(url, "application/json", strings.NewReader(string(jsonPayload)))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to send message to Telegram: %s", resp.Status)
	}

	return nil
}

func main() {
	file, err := os.OpenFile("jupyterlabs.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}
	defer file.Close()

	output := bufio.NewWriter(file)
	defer output.Flush()

	scanner := bufio.NewScanner(os.Stdin)
	total := 0
	found := 0

	for scanner.Scan() {
		ip := strings.TrimSpace(scanner.Text())
		if ip == "" {
			continue
		}

		total++
		if total%100 == 0 {
			fmt.Printf("\r%d scanned, %d jupyter", total, found)
		}

		if checkJupyterLab(ip) {
			result := fmt.Sprintf("http://%s:8888\n", ip)
			output.WriteString(result)
			output.Flush()
			fmt.Printf("\n[+] %s", result)
			found++

			// Отправляем результат в Telegram
			err := sendToTelegram(ip)
			if err != nil {
				fmt.Printf("Failed to send IP to Telegram: %v\n", err)
			} else {
				fmt.Printf("Sent IP to Telegram successfully.\n")
			}
		}
	}

	fmt.Printf("\n\nDone. Total: %d, JupyterLab found: %d\n", total, found)

	if err := scanner.Err(); err != nil {
		fmt.Printf("error: %v\n", err)
	}
}
