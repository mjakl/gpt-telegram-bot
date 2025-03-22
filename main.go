package main

import (
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/sashabaranov/go-openai"
	"log"
	"openrouter-gpt-telegram-bot/api"
	"openrouter-gpt-telegram-bot/config"
	"openrouter-gpt-telegram-bot/lang"
	"openrouter-gpt-telegram-bot/user"
	"strconv"
)

// sendMessage sends a message to the specified chat
func sendMessage(bot *tgbotapi.BotAPI, chatID int64, text string, parseMode string) error {
	msg := tgbotapi.NewMessage(chatID, text)
	if parseMode != "" {
		msg.ParseMode = parseMode
	}
	_, err := bot.Send(msg)
	return err
}

// handleUserMessage processes a user message and gets a response from the AI
func handleUserMessage(bot *tgbotapi.BotAPI, client *openai.Client, message *tgbotapi.Message, conf *config.Config, userStats *user.UsageTracker) {
	log.Printf("Processing message from User ID: %d, Username: %s, Chat ID: %d, Message: %s",
		message.From.ID,
		message.From.UserName,
		message.Chat.ID,
		message.Text)
		
	if !userStats.HaveAccess(conf) {
		log.Printf("User ID: %d has no budget access", message.From.ID)
		err := sendMessage(bot, message.Chat.ID, lang.Translate("budget_out", conf.Lang), "")
		if err != nil {
			log.Println(err)
		}
		return
	}
	
	log.Printf("Sending request to API for User ID: %d", message.From.ID)
	responseID := api.HandleChatGPTStreamResponse(bot, client, message, conf, userStats)
	log.Printf("Received response ID: %s for User ID: %d", responseID, message.From.ID)
	
	if conf.Model.Type == "openrouter" {
		userStats.GetUsageFromApi(responseID, conf)
	}
}

func main() {
	err := lang.LoadTranslations("./lang/")
	if err != nil {
		log.Fatalf("Error loading translations: %v", err)
	}

	manager, err := config.NewManager("./config.yaml") // or the path to your config file
	if err != nil {
		log.Fatalf("Error initializing config manager: %v", err)
	}

	conf := manager.GetConfig()

	bot, err := tgbotapi.NewBotAPI(conf.TelegramBotToken)
	if err != nil {
		log.Panic(err)
	}
	bot.Debug = false

	// Delete the webhook
	_, err = bot.Request(tgbotapi.DeleteWebhookConfig{})
	if err != nil {
		log.Fatalf("Failed to delete webhook: %v", err)
	}

	// Now you can safely use getUpdates
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	//Set bot commands
	commands := []tgbotapi.BotCommand{
		{Command: "start", Description: lang.Translate("description.start", conf.Lang)},
		{Command: "help", Description: lang.Translate("description.help", conf.Lang)},
		{Command: "reset", Description: lang.Translate("description.reset", conf.Lang)},
		{Command: "stats", Description: lang.Translate("description.stats", conf.Lang)},
		{Command: "stop", Description: lang.Translate("description.stop", conf.Lang)},
		{Command: "q", Description: lang.Translate("description.q", conf.Lang)},
	}
	_, err = bot.Request(tgbotapi.NewSetMyCommands(commands...))
	if err != nil {
		log.Fatalf("Failed to set bot commands: %v", err)
	}

	clientOptions := openai.DefaultConfig(conf.OpenAIApiKey)
	clientOptions.BaseURL = conf.OpenAIBaseURL
	client := openai.NewClientWithConfig(clientOptions)

	userManager := user.NewUserManager("logs")

	for update := range updates {
		if update.Message == nil {
			continue
		}
		userStats := userManager.GetUser(update.SentFrom().ID, update.SentFrom().UserName, conf)
		
		if update.Message.IsCommand() {
			switch update.Message.Command() {
			case "start":
				msgText := lang.Translate("commands.start", conf.Lang) + lang.Translate("commands.help", conf.Lang) + lang.Translate("commands.start_end", conf.Lang)
				sendMessage(bot, update.Message.Chat.ID, msgText, "HTML")
				
			case "help":
				sendMessage(bot, update.Message.Chat.ID, lang.Translate("commands.help", conf.Lang), "HTML")
				
			case "reset":
				args := update.Message.CommandArguments()
				var msgText string

				if args == "system" {
					userStats.SystemPrompt = conf.SystemPrompt
					msgText = lang.Translate("commands.reset_system", conf.Lang)
				} else if args != "" {
					userStats.SystemPrompt = args
					msgText = lang.Translate("commands.reset_prompt", conf.Lang) + args + "."
				} else {
					userStats.ClearHistory()
					msgText = lang.Translate("commands.reset", conf.Lang)
				}
				sendMessage(bot, update.Message.Chat.ID, msgText, "")
				
			case "stats":
				userStats.CheckHistory(conf.MaxHistorySize, conf.MaxHistoryTime)
				countedUsage := strconv.FormatFloat(userStats.GetCurrentCost(conf.BudgetPeriod), 'f', 6, 64)
				todayUsage := strconv.FormatFloat(userStats.GetCurrentCost("daily"), 'f', 6, 64)
				monthUsage := strconv.FormatFloat(userStats.GetCurrentCost("monthly"), 'f', 6, 64)
				totalUsage := strconv.FormatFloat(userStats.GetCurrentCost("total"), 'f', 6, 64)
				messagesCount := strconv.Itoa(len(userStats.GetMessages()))

				var statsMessage string
				if userStats.CanViewStats(conf) {
					statsMessage = fmt.Sprintf(
						lang.Translate("commands.stats", conf.Lang),
						countedUsage, todayUsage, monthUsage, totalUsage, messagesCount)
				} else {
					statsMessage = fmt.Sprintf(
						lang.Translate("commands.stats_min", conf.Lang), messagesCount)
				}

				sendMessage(bot, update.Message.Chat.ID, statsMessage, "HTML")

			case "stop":
				var msgText string
				if userStats.CurrentStream != nil {
					userStats.CurrentStream.Close()
					msgText = lang.Translate("commands.stop", conf.Lang)
				} else {
					msgText = lang.Translate("commands.stop_err", conf.Lang)
				}
				sendMessage(bot, update.Message.Chat.ID, msgText, "")
				
			case "q":
				args := update.Message.CommandArguments()
				if args == "" {
					sendMessage(bot, update.Message.Chat.ID, lang.Translate("commands.q_empty", conf.Lang), "")
					continue
				}
				
				// Trim quotes if present (handles both single and double quotes)
				if len(args) >= 2 && ((args[0] == '"' && args[len(args)-1] == '"') || (args[0] == '\'' && args[len(args)-1] == '\'')) {
					args = args[1 : len(args)-1]
				}
				
				// Log the /q command usage
				log.Printf("Question command received from User ID: %d, Username: %s, Question: %s", 
					update.Message.From.ID, 
					update.Message.From.UserName, 
					args)
				
				go func(userStats *user.UsageTracker) {
					// Create a new message with the command arguments
					questionMsg := tgbotapi.Message{
						Text: args,
						From: update.Message.From,
						Chat: update.Message.Chat,
					}
					
					handleUserMessage(bot, client, &questionMsg, conf, userStats)
				}(userStats)
			}
		} else {
			go func(userStats *user.UsageTracker) {
				handleUserMessage(bot, client, update.Message, conf, userStats)
			}(userStats)
		}
	}
}
