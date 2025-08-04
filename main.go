package main

import (
	"bytes"
	"fmt"
	"gocv.io/x/gocv"
	"gopkg.in/telebot.v3"
	"gopkg.in/yaml.v2"
	"io"
	"log"
	"os"
	"time"
)

type Config struct {
	Telegram struct {
		BotToken     string  `yaml:"bot_token"`
		WelcomeMsg   string  `yaml:"welcome_msg"`
		AllowedUsers []int64 `yaml:"allowed_users"`
	} `yaml:"telegram"`
	Camera struct {
		Filename string `yaml:"filename"`
		DeviceID int    `yaml:"device_id"`
	} `yaml:"camera"`
}

func loadConfig(path string) (Config, error) {
	var cfg Config
	file, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	err = yaml.Unmarshal(file, &cfg)
	return cfg, err
}
func takeScreenshot(deviceID int, filename string) error {
	webcam, err := gocv.OpenVideoCapture(deviceID)
	if err != nil {
		return fmt.Errorf("камера %d не открылась: %w", deviceID, err)
	}
	defer webcam.Close()

	img := gocv.NewMat()
	defer img.Close()

	if ok := webcam.Read(&img); !ok || img.Empty() {
		return fmt.Errorf("не удалось захватить изображение с камеры %d", deviceID)
	}

	if ok := gocv.IMWrite(filename, img); !ok {
		return fmt.Errorf("не удалось сохранить изображение")
	}

	return nil
}

func isUserAllowed(userID int64, allowed []int64) bool {
	for _, id := range allowed {
		if userID == id {
			return true
		}
	}
	return false
}

func main() {
	cfg, err := loadConfig("config.yml")
	if err != nil {
		log.Fatalf("Ошибка загрузки config.yml: %v", err)
	}

	if cfg.Telegram.BotToken == "" {
		log.Fatal("bot_token не задан в config.yml")
	}

	bot, err := telebot.NewBot(telebot.Settings{
		Token:  cfg.Telegram.BotToken,
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		log.Fatalf("Ошибка создания бота: %v", err)
	}

	deny := func(c telebot.Context) error {
		log.Printf("Доступ запрещён: %d", c.Sender().ID)
		return c.Send("🚫 У вас нет доступа к этому боту.")
	}

	bot.Handle("/start", func(c telebot.Context) error {
		if !isUserAllowed(c.Sender().ID, cfg.Telegram.AllowedUsers) {
			return deny(c)
		}
		return c.Send(cfg.Telegram.WelcomeMsg)
	})

	bot.Handle("/photo", func(c telebot.Context) error {
		if !isUserAllowed(c.Sender().ID, cfg.Telegram.AllowedUsers) {
			return deny(c)
		}

		filename := cfg.Camera.Filename
		deviceID := cfg.Camera.DeviceID

		err := takeScreenshot(deviceID, filename)
		if err != nil {
			log.Printf("Ошибка скриншота: %v", err)
			return c.Send("Ошибка захвата изображения с камеры.")
		}

		photo := &telebot.Photo{}
		file, err := os.Open(filename)
		if err != nil {
			return c.Send("Ошибка открытия изображения.")
		}
		defer file.Close()

		buf := new(bytes.Buffer)
		io.Copy(buf, file)

		photo.File = telebot.FromReader(buf)
		photo.Caption = "📸 Вот ваш снимок с камеры"

		return c.Send(photo)
	})

	log.Println("Бот запущен")
	bot.Start()
}
