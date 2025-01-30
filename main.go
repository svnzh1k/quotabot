package main

import (
	"encoding/json"
	"log"
	"os"
	"tgbot/reg"
	"tgbot/telegram"
)

type Config struct {
	Token      string   `json:"token"`
	Subscriber int      `json:"subscriber"`
	Cookie     string   `json:"cookie"`
	Courses    []string `json:"courses"`
	Timeout    int      `json:"timeout"`
}

func main() {
	// execPath, err := os.Executable()
	// if err != nil {
	// 	log.Fatalf("Error getting executable path: %v", err)
	// }
	// execDir := filepath.Dir(execPath)

	// configFilePath := filepath.Join(execDir, "config.json")
	file, err := os.Open("config.json")
	if err != nil {
		log.Fatalf("Error opening config.json: %v", err)
	}
	defer file.Close()

	var config Config
	if err := json.NewDecoder(file).Decode(&config); err != nil {
		log.Fatalf("Error decoding config.json: %v", err)
	}

	if len(config.Courses) > 10 || len(config.Courses) < 1 {
		log.Fatal("Количество предметов должно быть от 1 до 10")
	}

	requesters := make([]reg.Requester, len(config.Courses))
	for i, courseCode := range config.Courses {
		requesters[i] = reg.New(config.Cookie, courseCode)
	}

	bot := telegram.NewBot(config.Token, requesters, config.Subscriber)

	go bot.Process()
	bot.StartQuotaChecker(config.Timeout)
}
