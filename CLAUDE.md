# CLAUDE.md - Openrouter GPT Telegram Bot

## Build & Run Commands
- Build: `go build -o openrouter-gpt-telegram-bot`
- Run: `go run main.go`
- Docker build and run: `docker compose up`
- Tests: No test commands found in the codebase

## Code Style Guidelines
- **Imports**: Group standard library imports first, then external packages, then local packages
- **Formatting**: Use `gofmt` for standard Go formatting
- **Error Handling**: Use explicit error checks after each function call that returns an error
- **Logging**: Use the standard `log` package for logging errors and important information
- **Variable Naming**: Use camelCase for variables, PascalCase for exported functions/variables
- **Config Management**: Use `config` package with Viper for configuration management
- **Language Support**: Use the `lang` package for internationalization
- **User Management**: Use the `user` package for tracking users and their usage
- **API Handling**: Use the `api` package for communicating with OpenRouter/OpenAI APIs

## Project Structure
- Configuration files in `config.yaml` (renamed from example.env)
- Language files in `lang/` directory (EN.json, RU.json)
- User tracking in `user/` directory
- API integration in `api/` directory