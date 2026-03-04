package bot

import (
	"fmt"
	"html"
	"strconv"
	"strings"
	"time"

	"hadith-bot/internal/image"
	"hadith-bot/internal/logger"
	"hadith-bot/internal/models"
	"hadith-bot/internal/services"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const telegramMessageMaxRunes = 3800

type Handler struct {
	bot                 *tgbotapi.BotAPI
	hadithService       *services.HadithService
	log                 *logger.Logger
	rateLimiter         *RateLimiter
	imageGenerator      *image.Generator
	state               *StateManager
	imageCacheChannelID int64
}

func NewHandler(bot *tgbotapi.BotAPI, hadithService *services.HadithService, log *logger.Logger, rateLimitRequests int, rateLimitWindow time.Duration, imageGenerator *image.Generator, state *StateManager, imageCacheChannelID int64) *Handler {
	return &Handler{
		bot:                 bot,
		hadithService:       hadithService,
		log:                 log,
		rateLimiter:         NewRateLimiter(rateLimitRequests, rateLimitWindow),
		imageGenerator:      imageGenerator,
		state:               state,
		imageCacheChannelID: imageCacheChannelID,
	}
}

func (h *Handler) StartScheduler() {
	ticker := time.NewTicker(1 * time.Minute)
	go func() {
		for range ticker.C {
			h.processSchedules()
		}
	}()
}

func (h *Handler) processSchedules() {
	states := h.state.GetAll()
	now := time.Now()

	for chatID, chatState := range states {
		if chatState.ScheduleInterval > 0 && now.Sub(chatState.LastSentAt) >= chatState.ScheduleInterval {
			// Update the time first to prevent double-sending if this takes a while
			chatState.LastSentAt = now
			h.state.SetChatState(chatID, chatState)

			// Generate and send random hadith image
			res := h.hadithService.GetRandomHadith()
			if res.Hadith == nil || res.Collection == nil {
				continue // skip if we fail to fetch a random hadith
			}

			book := h.hadithService.GetBook(res.Collection.Name, res.Hadith.ChapterID)
			title := "Hadith"
			if book != nil {
				title = book.Title
				if idx := strings.Index(title, ". "); idx != -1 {
					title = title[idx+2:]
				}
			}

			ref := fmt.Sprintf("[%s: %d]", services.GetCollectionDisplayName(res.Collection.Name), res.Hadith.HadithNumber)

			imgBytes, err := h.imageGenerator.GenerateHadithImage(title, res.Hadith.Narrator, res.Hadith.Arabic, res.Hadith.English, ref, chatState.UseCustomBg)
			if err != nil {
				h.log.Error("Failed to generate scheduled image for %d: %v", chatID, err)
				continue
			}

			photo := tgbotapi.NewPhoto(chatID, tgbotapi.FileBytes{
				Name:  "hadith.png",
				Bytes: imgBytes,
			})
			_, err = h.bot.Send(photo)

			// If sending the photo fails (e.g., media disabled in group), fallback to text mode
			if err != nil {
				h.log.Error("Failed to send scheduled image for %d (falling back to text): %v", chatID, err)
				h.sendRandomHadithPaged(chatID, 0, "", res.Collection.Name, res.Hadith.HadithNumber, 0)
			}
		}
	}
}

func (h *Handler) StartListening() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := h.bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message != nil {
			h.handleIncomingMessage(update.Message)
		} else if update.CallbackQuery != nil {
			h.handleCallback(update.CallbackQuery)
		} else if update.InlineQuery != nil {
			h.handleInlineQuery(update.InlineQuery)
		}
	}
}

func (h *Handler) handleIncomingMessage(m *tgbotapi.Message) {
	if !h.rateLimiter.Allow(m.From.ID) {
		h.sendMessage(m.Chat.ID, "⏳ Please wait a moment before sending another command.")
		return
	}

	if m.IsCommand() {
		switch m.Command() {
		case "start":
			h.handleStart(m)
		case "help":
			h.handleHelp(m)
		case "random":
			h.handleRandom(m)
		case "search":
			h.handleSearch(m)
		case "collections":
			h.handleCollections(m)
		case "togglebackgrounds":
			h.handleToggleBackgrounds(m)
		case "schedule":
			h.handleSchedule(m)
		}
	}
}

// --- COMMAND HANDLERS ---

func (h *Handler) handleStart(m *tgbotapi.Message) {
	chatState := h.state.GetChatState(m.Chat.ID)
	if chatState == nil {
		chatState = &ChatState{
			ScheduleInterval: 6 * time.Hour,
			LastSentAt:       time.Now(),
		}
		h.state.SetChatState(m.Chat.ID, chatState)
	}

	args := m.CommandArguments()
	if strings.HasPrefix(args, "hadith_") {
		parts := strings.Split(args, "_")
		if len(parts) >= 3 {
			col := parts[1]
			hadithNum, _ := strconv.Atoi(parts[2])
			h.sendSearchHadithPaged(m.Chat.ID, 0, "", col, hadithNum, 0)
			return
		}
	}

	text := `🕌 <b>Welcome to Hadith Portal Bot</b>

Explore authentic hadith from major collections with a clean, simple menu.

✨ <b>Quick actions:</b> use the buttons below to browse, search, or get a random hadith.`

	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📚 Browse Collections", "collections:1"),
			tgbotapi.NewInlineKeyboardButtonData("🔍 Search Hadith", "search"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🎲 Random Hadith", "random"),
			tgbotapi.NewInlineKeyboardButtonData("❓ Help", "help"),
		),
	)
	h.sendMessageWithKeyboard(m.Chat.ID, text, kb)
}

func (h *Handler) handleHelp(m *tgbotapi.Message) {
	helpText := `❓ <b>Hadith Portal Bot — Help</b>

<b>Commands</b>
• <b>/start</b> — Open the main menu
• <b>/collections</b> — Browse hadith collections
• <b>/search &lt;keyword&gt;</b> — Search hadith text
• <b>/random</b> — Get a random hadith
• <b>/togglebackgrounds</b> — Toggle custom image backgrounds for generated images
• <b>/help</b> — Show this help message

💡 <b>Examples</b>
• <b>/search prayer</b>
• <b>/search patience</b>`
	h.sendMessage(m.Chat.ID, helpText)
}

func (h *Handler) handleRandom(m *tgbotapi.Message) {
	res := h.hadithService.GetRandomHadith()
	if res.Hadith == nil || res.Collection == nil {
		h.sendMessage(m.Chat.ID, "⚠️ Could not fetch a hadith right now. Please try again.")
		return
	}
	h.sendRandomHadithPaged(m.Chat.ID, 0, "", res.Collection.Name, res.Hadith.HadithNumber, 0)
}

func (h *Handler) handleSearch(m *tgbotapi.Message) {
	args := m.CommandArguments()
	if args == "" {
		h.sendMessage(m.Chat.ID, "🔎 Please provide a keyword. Example: <b>/search prayer</b>")
		return
	}
	results := h.hadithService.SearchHadiths(args, 1, 10)
	if len(results.Hadiths) == 0 {
		h.sendMessage(m.Chat.ID, "No results found for your search. Try a different keyword.")
		return
	}
	h.sendSearchResults(m.Chat.ID, 0, "", args, results)
}

func (h *Handler) handleCollections(m *tgbotapi.Message) {
	h.sendCollectionsMenu(m.Chat.ID, 0, "", h.hadithService.GetCollections(), 1)
}

func (h *Handler) handleSchedule(m *tgbotapi.Message) {
	if m.Chat.IsGroup() || m.Chat.IsSuperGroup() {
		member, err := h.bot.GetChatMember(tgbotapi.GetChatMemberConfig{
			ChatConfigWithUser: tgbotapi.ChatConfigWithUser{
				ChatID: m.Chat.ID,
				UserID: m.From.ID,
			},
		})

		if err != nil || (!member.IsCreator() && !member.IsAdministrator()) {
			h.sendMessage(m.Chat.ID, "⚠️ Only group administrators can change the schedule.")
			return
		}
	}

	args := strings.TrimSpace(m.CommandArguments())
	chatState := h.state.GetChatState(m.Chat.ID)
	if chatState == nil {
		chatState = &ChatState{
			LastSentAt: time.Now(),
		}
	}

	if args == "" {
		if chatState.ScheduleInterval > 0 {
			h.sendMessage(m.Chat.ID, fmt.Sprintf("🕒 Current schedule is set to **%v**.\n\nUse `/schedule off` to disable, or `/schedule <duration>` to change (e.g., `2h`, `12h`).", chatState.ScheduleInterval))
		} else {
			h.sendMessage(m.Chat.ID, "🕒 There is currently no active schedule.\n\nUse `/schedule <duration>` to enable (e.g., `2h`, `6h`, `12h`).")
		}
		return
	}

	if strings.ToLower(args) == "off" {
		chatState.ScheduleInterval = 0
		h.state.SetChatState(m.Chat.ID, chatState)
		h.sendMessage(m.Chat.ID, "✅ Automatic scheduled messages have been turned **OFF**.")
		return
	}

	d, err := time.ParseDuration(args)
	if err != nil || d <= 0 {
		h.sendMessage(m.Chat.ID, "⚠️ Invalid duration format. Please use something like `2h`, `6h`, or `12h`.")
		return
	}

	chatState.ScheduleInterval = d
	// Reset the timer when they set a new schedule
	chatState.LastSentAt = time.Now()
	h.state.SetChatState(m.Chat.ID, chatState)

	h.sendMessage(m.Chat.ID, fmt.Sprintf("✅ Schedule updated! A random hadith image will be sent every **%v**.", d))
}

func (h *Handler) handleToggleBackgrounds(m *tgbotapi.Message) {
	userID := m.From.ID

	chatState := h.state.GetChatState(userID)
	if chatState == nil {
		chatState = &ChatState{}
	}

	newSetting := !chatState.UseCustomBg
	chatState.UseCustomBg = newSetting

	h.state.SetChatState(userID, chatState)

	var status string
	if newSetting {
		status = "ON 🎨 (Custom Image Backgrounds)"
	} else {
		status = "OFF 📜 (Default Pattern Background)"
	}

	text := fmt.Sprintf("✅ Custom backgrounds are now <b>%s</b> for generated images.", status)
	h.sendMessage(m.Chat.ID, text)
}

// --- CALLBACK HANDLER ---

func (h *Handler) handleCallback(c *tgbotapi.CallbackQuery) {
	if !h.rateLimiter.Allow(c.From.ID) {
		h.bot.Request(tgbotapi.NewCallback(c.ID, "⏳ Slow down a little."))
		return
	}

	parts := strings.Split(c.Data, ":")
	var chatID int64
	var msgID int
	if c.Message != nil {
		chatID = c.Message.Chat.ID
		msgID = c.Message.MessageID
	}
	iMID := c.InlineMessageID

	switch parts[0] {
	case "collections":
		page, _ := strconv.Atoi(parts[1])
		h.sendCollectionsMenu(chatID, msgID, iMID, h.hadithService.GetCollections(), page)
	case "books":
		colName := parts[1]
		page, _ := strconv.Atoi(parts[2])
		h.sendBooksMenu(chatID, msgID, iMID, colName, h.hadithService.GetBooks(colName), page)
	case "hadiths":
		colName := parts[1]
		bookNum, _ := strconv.Atoi(parts[2])
		page, _ := strconv.Atoi(parts[3])
		res := h.hadithService.GetHadiths(colName, bookNum, page, 10)
		h.sendHadithsMenu(chatID, msgID, iMID, colName, bookNum, res)
	case "hadith_detail":
		h.handleHadithDetailCallback(c, parts)
	case "hadith_search":
		h.handleHadithSearchCallback(c, parts)
	case "hadith_page":
		h.handleHadithPageCallback(c, parts)
	case "random":
		h.handleRandomCallback(c)
	case "search_next", "search_prev":
		query := parts[1]
		page, _ := strconv.Atoi(parts[2])
		res := h.hadithService.SearchHadiths(query, page, 5)
		h.sendSearchResults(chatID, msgID, iMID, query, res)
	case "help":
		h.sendMessage(chatID, "Use <b>/help</b> to view all commands and examples.")
	case "hadith_image":
		h.handleHadithImageCallback(c, parts)
	}

	h.bot.Request(tgbotapi.NewCallback(c.ID, ""))
}

// --- INLINE QUERY HANDLER ---

func (h *Handler) handleInlineQuery(q *tgbotapi.InlineQuery) {
	query := strings.TrimSpace(q.Query)
	var results []interface{}

	if query == "" || query == "collections" {
		text := "📚 <b>Browse Hadith Collections</b>\nSelect a collection to view its books and hadiths."
		article := tgbotapi.NewInlineQueryResultArticleHTML(q.ID+"_browse", "📚 Browse Collections", text)
		article.Description = "View Bukhari, Muslim, and other major collections"
		kb := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("📚 Open Collections", "collections:1"),
				tgbotapi.NewInlineKeyboardButtonData("🎲 Random Hadith", "random"),
			),
		)
		article.ReplyMarkup = &kb
		results = append(results, article)
	}

	if query == "random" {
		res := h.hadithService.GetRandomHadith()
		if res.Hadith != nil && res.Collection != nil {
			txt := h.formatHadithDisplay(res.Hadith, res.Collection, res.Book)
			pages := splitTelegramMessage(txt, telegramMessageMaxRunes)
			if len(pages) == 0 {
				pages = []string{txt}
			}

			display := pages[0]
			if len(pages) > 1 {
				display = fmt.Sprintf("<b>Page 1/%d</b>\n\n%s", len(pages), display)
			}

			article := tgbotapi.NewInlineQueryResultArticleHTML(q.ID, "🎲 Random Hadith", txt)
			article.Description = fmt.Sprintf("Hadith #%d from %s", res.Hadith.HadithNumber, getCollectionTitle(res.Collection))
			article.InputMessageContent = tgbotapi.InputTextMessageContent{
				Text:      display,
				ParseMode: tgbotapi.ModeHTML,
			}

			var rows [][]tgbotapi.InlineKeyboardButton
			if len(pages) > 1 {
				rows = append(rows, tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("Next Part ➡️", fmt.Sprintf("hadith_page:r:%s:%d:%d", res.Collection.Name, res.Hadith.HadithNumber, 1)),
				))
			}
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🎲 Another Random", "random"),
			))
			kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
			article.ReplyMarkup = &kb
			results = append(results, article)
		}
	} else if strings.HasPrefix(query, "image-search ") {
		keyword := strings.TrimSpace(strings.TrimPrefix(query, "image-search "))
		if keyword != "" {
			if h.imageCacheChannelID == 0 {
				article := tgbotapi.NewInlineQueryResultArticle(q.ID+"_err", "⚠️ Image Search Unavailable", "The bot administrator has not configured an image cache channel. Image search is currently disabled.")
				results = append(results, article)
			} else {
				searchRes := h.hadithService.SearchHadiths(keyword, 1, 1) // only top 1 result
				if len(searchRes.Hadiths) > 0 {
					hadith := searchRes.Hadiths[0]
					colName := h.findCollectionForHadith(hadith)
					book := h.hadithService.GetBook(colName, hadith.ChapterID)

					title := "Hadith"
					if book != nil {
						title = book.Title
						if idx := strings.Index(title, ". "); idx != -1 {
							title = title[idx+2:]
						}
					}
					ref := fmt.Sprintf("[%s: %d]", services.GetCollectionDisplayName(colName), hadith.HadithNumber)

					useCustomBg := false
					if chatState := h.state.GetChatState(int64(q.From.ID)); chatState != nil {
						useCustomBg = chatState.UseCustomBg
					}

					imgBytes, err := h.imageGenerator.GenerateHadithImage(title, hadith.Narrator, hadith.Arabic, hadith.English, ref, useCustomBg)
					if err == nil {
						photoMsg := tgbotapi.NewPhoto(h.imageCacheChannelID, tgbotapi.FileBytes{
							Name:  "hadith.png",
							Bytes: imgBytes,
						})

						sentMsg, err := h.bot.Send(photoMsg)
						if err == nil && len(sentMsg.Photo) > 0 {
							// get the largest photo
							largestPhoto := sentMsg.Photo[len(sentMsg.Photo)-1]
							fileID := largestPhoto.FileID

							photoResult := tgbotapi.NewInlineQueryResultCachedPhoto(q.ID+"_img", fileID)
							results = append(results, photoResult)
						} else {
							h.log.Error("Failed to send image to cache channel: %v", err)
						}
					} else {
						h.log.Error("Failed to generate image for inline query: %v", err)
					}
				}

				if len(results) == 0 {
					article := tgbotapi.NewInlineQueryResultArticle(q.ID+"_nores", "No results found", "Could not find any hadiths matching your query or an error occurred.")
					results = append(results, article)
				}
			}
		}
	} else if strings.HasPrefix(query, "search ") {
		keyword := strings.TrimSpace(strings.TrimPrefix(query, "search "))
		if keyword != "" {
			searchRes := h.hadithService.SearchHadiths(keyword, 1, 5)
			for i, hadith := range searchRes.Hadiths {
				colName := h.findCollectionForHadith(hadith)
				col := h.hadithService.GetCollection(colName)
				txt := h.formatHadithDisplay(&hadith, col, nil)
				pages := splitTelegramMessage(txt, telegramMessageMaxRunes)
				if len(pages) == 0 {
					pages = []string{txt}
				}

				display := pages[0]
				if len(pages) > 1 {
					display = fmt.Sprintf("<b>Page 1/%d</b>\n\n%s", len(pages), display)
				}

				id := fmt.Sprintf("inline_%d_%d", hadith.HadithNumber, i)
				article := tgbotapi.NewInlineQueryResultArticleHTML(id, fmt.Sprintf("🵿 Hadith #%d", hadith.HadithNumber), txt)
				article.Description = truncate(hadith.English, 50)
				article.InputMessageContent = tgbotapi.InputTextMessageContent{
					Text:      display,
					ParseMode: tgbotapi.ModeHTML,
				}

				var rows [][]tgbotapi.InlineKeyboardButton
				if len(pages) > 1 {
					rows = append(rows, tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData("Next Part ➡️", fmt.Sprintf("hadith_page:s:%s:%d:%d", colName, hadith.HadithNumber, 1)),
					))
				}
				rows = append(rows, tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("Open in Bot", fmt.Sprintf("hadith_search:%s:%d", colName, hadith.HadithNumber)),
				))
				kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
				article.ReplyMarkup = &kb
				results = append(results, article)
			}
		}
	}

	h.bot.Request(tgbotapi.InlineConfig{
		InlineQueryID: q.ID,
		Results:       results,
		CacheTime:     10,
	})
}

// --- NAVIGATION MENUS ---

func (h *Handler) sendCollectionsMenu(chatID int64, msgID int, inlineMsgID string, collections []models.Collection, page int) {
	const perPage = 6
	start, end := (page-1)*perPage, page*perPage
	if end > len(collections) {
		end = len(collections)
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	for _, c := range collections[start:end] {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData(c.Title, fmt.Sprintf("books:%s:1", c.Name))))
	}

	var nav []tgbotapi.InlineKeyboardButton
	if page > 1 {
		nav = append(nav, tgbotapi.NewInlineKeyboardButtonData("⬅️ Prev", fmt.Sprintf("collections:%d", page-1)))
	}
	if end < len(collections) {
		nav = append(nav, tgbotapi.NewInlineKeyboardButtonData("Next ➡️", fmt.Sprintf("collections:%d", page+1)))
	}
	if len(nav) > 0 {
		rows = append(rows, nav)
	}

	h.editOrSendMessage(chatID, msgID, inlineMsgID, "📚 <b>Select a Collection:</b>", tgbotapi.NewInlineKeyboardMarkup(rows...))
}

func (h *Handler) sendBooksMenu(chatID int64, msgID int, inlineMsgID string, col string, books []models.Book, page int) {
	const perPage = 10
	start, end := (page-1)*perPage, page*perPage
	if end > len(books) {
		end = len(books)
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	for _, b := range books[start:end] {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData(truncate(b.Title, 35), fmt.Sprintf("hadiths:%s:%d:1", col, b.BookNumber))))
	}

	var nav []tgbotapi.InlineKeyboardButton
	if page > 1 {
		nav = append(nav, tgbotapi.NewInlineKeyboardButtonData("⬅️ Page", fmt.Sprintf("books:%s:%d", col, page-1)))
	}
	if end < len(books) {
		nav = append(nav, tgbotapi.NewInlineKeyboardButtonData("Page ➡️", fmt.Sprintf("books:%s:%d", col, page+1)))
	}
	if len(nav) > 0 {
		rows = append(rows, nav)
	}

	rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("⬅️ Back to Collections", "collections:1")))
	h.editOrSendMessage(chatID, msgID, inlineMsgID, fmt.Sprintf("📚 <b>%s — Books</b>", html.EscapeString(services.GetCollectionDisplayName(col))), tgbotapi.NewInlineKeyboardMarkup(rows...))
}

func (h *Handler) sendHadithsMenu(chatID int64, msgID int, inlineMsgID string, col string, bookNum int, result models.HadithResponse) {
	var rows [][]tgbotapi.InlineKeyboardButton
	for i, hadith := range result.Hadiths {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("📜 Hadith #%d", hadith.HadithNumber), fmt.Sprintf("hadith_detail:%s:%d:%d:%d", col, bookNum, result.Page, i))))
	}

	var nav []tgbotapi.InlineKeyboardButton
	if result.Page > 1 {
		nav = append(nav, tgbotapi.NewInlineKeyboardButtonData("⬅️ Prev", fmt.Sprintf("hadiths:%s:%d:%d", col, bookNum, result.Page-1)))
	}
	if result.Page < result.TotalPages {
		nav = append(nav, tgbotapi.NewInlineKeyboardButtonData("Next ➡️", fmt.Sprintf("hadiths:%s:%d:%d", col, bookNum, result.Page+1)))
	}
	if len(nav) > 0 {
		rows = append(rows, nav)
	}

	rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("⬅️ Back to Books", fmt.Sprintf("books:%s:1", col))))
	h.editOrSendMessage(chatID, msgID, inlineMsgID, fmt.Sprintf("📑 <b>Hadith List — Page %d/%d</b>", result.Page, result.TotalPages), tgbotapi.NewInlineKeyboardMarkup(rows...))
}

func (h *Handler) handleHadithDetailCallback(c *tgbotapi.CallbackQuery, parts []string) {
	col, bookNum, page, index := parts[1], 0, 0, 0
	fmt.Sscanf(parts[2], "%d", &bookNum)
	fmt.Sscanf(parts[3], "%d", &page)
	fmt.Sscanf(parts[4], "%d", &index)

	chatID := int64(0)
	msgID := 0
	if c.Message != nil {
		chatID = c.Message.Chat.ID
		msgID = c.Message.MessageID
	}
	h.sendHadithDetailPaged(chatID, msgID, c.InlineMessageID, col, bookNum, page, index, 0)
}

func (h *Handler) handleHadithPageCallback(c *tgbotapi.CallbackQuery, parts []string) {
	if len(parts) < 3 {
		return
	}

	chatID := int64(0)
	msgID := 0
	if c.Message != nil {
		chatID = c.Message.Chat.ID
		msgID = c.Message.MessageID
	}

	switch parts[1] {
	case "d":
		if len(parts) < 7 {
			return
		}
		col := parts[2]
		bookNum, _ := strconv.Atoi(parts[3])
		listPage, _ := strconv.Atoi(parts[4])
		index, _ := strconv.Atoi(parts[5])
		textPage, _ := strconv.Atoi(parts[6])
		h.sendHadithDetailPaged(chatID, msgID, c.InlineMessageID, col, bookNum, listPage, index, textPage)
	case "s":
		if len(parts) < 5 {
			return
		}
		col := parts[2]
		hadithNum, _ := strconv.Atoi(parts[3])
		textPage, _ := strconv.Atoi(parts[4])
		h.sendSearchHadithPaged(chatID, msgID, c.InlineMessageID, col, hadithNum, textPage)
	case "r":
		if len(parts) < 5 {
			return
		}
		col := parts[2]
		hadithNum, _ := strconv.Atoi(parts[3])
		textPage, _ := strconv.Atoi(parts[4])
		h.sendRandomHadithPaged(chatID, msgID, c.InlineMessageID, col, hadithNum, textPage)
	}
}

func (h *Handler) sendHadithDetailPaged(chatID int64, msgID int, inlineMsgID, col string, bookNum, page, index, textPage int) {

	res := h.hadithService.GetHadiths(col, bookNum, page, 10)
	if index < len(res.Hadiths) {
		hadith := res.Hadiths[index]
		txt := h.formatHadithDisplay(&hadith, h.hadithService.GetCollection(col), h.hadithService.GetBook(col, bookNum))
		pages := splitTelegramMessage(txt, telegramMessageMaxRunes)
		if len(pages) == 0 {
			pages = []string{txt}
		}

		if textPage < 0 {
			textPage = 0
		}
		if textPage >= len(pages) {
			textPage = len(pages) - 1
		}

		display := pages[textPage]
		if len(pages) > 1 {
			display = fmt.Sprintf("<b>Page %d/%d</b>\n\n%s", textPage+1, len(pages), display)
		}

		var rows [][]tgbotapi.InlineKeyboardButton
		if len(pages) > 1 {
			var nav []tgbotapi.InlineKeyboardButton
			if textPage > 0 {
				nav = append(nav, tgbotapi.NewInlineKeyboardButtonData("⬅️ Prev Part", fmt.Sprintf("hadith_page:d:%s:%d:%d:%d:%d", col, bookNum, page, index, textPage-1)))
			}
			if textPage < len(pages)-1 {
				nav = append(nav, tgbotapi.NewInlineKeyboardButtonData("Next Part ➡️", fmt.Sprintf("hadith_page:d:%s:%d:%d:%d:%d", col, bookNum, page, index, textPage+1)))
			}
			if len(nav) > 0 {
				rows = append(rows, nav)
			}
		}

		shareURL := fmt.Sprintf("https://t.me/%s?start=hadith_%s_%d", h.bot.Self.UserName, col, hadith.HadithNumber)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⬅️ Back", fmt.Sprintf("hadiths:%s:%d:%d", col, bookNum, page)),
			tgbotapi.NewInlineKeyboardButtonData("🎨 Image", fmt.Sprintf("hadith_image:%s:%d", col, hadith.HadithNumber)),
			tgbotapi.NewInlineKeyboardButtonURL("📤 Share", shareURL),
		))
		h.editOrSendMessage(chatID, msgID, inlineMsgID, display, tgbotapi.NewInlineKeyboardMarkup(rows...))
	}
}

func (h *Handler) handleHadithSearchCallback(c *tgbotapi.CallbackQuery, parts []string) {
	colName := parts[1]
	hadithNum, _ := strconv.Atoi(parts[2])

	chatID := int64(0)
	msgID := 0
	if c.Message != nil {
		chatID = c.Message.Chat.ID
		msgID = c.Message.MessageID
	}
	h.sendSearchHadithPaged(chatID, msgID, c.InlineMessageID, colName, hadithNum, 0)
}

func (h *Handler) sendSearchHadithPaged(chatID int64, msgID int, inlineMsgID, colName string, hadithNum, textPage int) {
	hadith, book := h.hadithService.FindHadithByNumber(colName, hadithNum)
	if hadith == nil {
		var rows [][]tgbotapi.InlineKeyboardButton
		if inlineMsgID == "" {
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🔍 New Search", "search"),
			))
		}

		h.editOrSendMessage(chatID, msgID, inlineMsgID, "⚠️ Could not find this hadith. Please try searching again.", tgbotapi.NewInlineKeyboardMarkup(rows...))
		return
	}

	txt := h.formatHadithDisplay(hadith, h.hadithService.GetCollection(colName), book)
	pages := splitTelegramMessage(txt, telegramMessageMaxRunes)
	if len(pages) == 0 {
		pages = []string{txt}
	}

	if textPage < 0 {
		textPage = 0
	}
	if textPage >= len(pages) {
		textPage = len(pages) - 1
	}

	display := pages[textPage]
	if len(pages) > 1 {
		display = fmt.Sprintf("<b>Page %d/%d</b>\n\n%s", textPage+1, len(pages), display)
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	if len(pages) > 1 {
		var nav []tgbotapi.InlineKeyboardButton
		if textPage > 0 {
			nav = append(nav, tgbotapi.NewInlineKeyboardButtonData("⬅️ Prev Part", fmt.Sprintf("hadith_page:s:%s:%d:%d", colName, hadithNum, textPage-1)))
		}
		if textPage < len(pages)-1 {
			nav = append(nav, tgbotapi.NewInlineKeyboardButtonData("Next Part ➡️", fmt.Sprintf("hadith_page:s:%s:%d:%d", colName, hadithNum, textPage+1)))
		}
		if len(nav) > 0 {
			rows = append(rows, nav)
		}
	}

	shareURL := fmt.Sprintf("https://t.me/%s?start=hadith_%s_%d", h.bot.Self.UserName, colName, hadithNum)
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🎨 Image", fmt.Sprintf("hadith_image:%s:%d", colName, hadithNum)),
		tgbotapi.NewInlineKeyboardButtonURL("📤 Share", shareURL),
	))

	if inlineMsgID == "" {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔍 New Search", "search"),
			tgbotapi.NewInlineKeyboardButtonData("🎲 Random", "random"),
		))
	}

	h.editOrSendMessage(chatID, msgID, inlineMsgID, display, tgbotapi.NewInlineKeyboardMarkup(rows...))
}

func (h *Handler) handleRandomCallback(c *tgbotapi.CallbackQuery) {
	res := h.hadithService.GetRandomHadith()
	if res.Hadith != nil && res.Collection != nil {
		chatID := int64(0)
		msgID := 0
		if c.Message != nil {
			chatID = c.Message.Chat.ID
			msgID = c.Message.MessageID
		}
		h.sendRandomHadithPaged(chatID, msgID, c.InlineMessageID, res.Collection.Name, res.Hadith.HadithNumber, 0)
	}
}

func (h *Handler) sendRandomHadithPaged(chatID int64, msgID int, inlineMsgID, colName string, hadithNum, textPage int) {
	hadith, book := h.hadithService.FindHadithByNumber(colName, hadithNum)
	if hadith == nil {
		h.sendMessage(chatID, "⚠️ Could not fetch a hadith right now. Please try again.")
		return
	}

	txt := h.formatHadithDisplay(hadith, h.hadithService.GetCollection(colName), book)
	pages := splitTelegramMessage(txt, telegramMessageMaxRunes)
	if len(pages) == 0 {
		pages = []string{txt}
	}

	if textPage < 0 {
		textPage = 0
	}
	if textPage >= len(pages) {
		textPage = len(pages) - 1
	}

	display := pages[textPage]
	if len(pages) > 1 {
		display = fmt.Sprintf("<b>Page %d/%d</b>\n\n%s", textPage+1, len(pages), display)
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	if len(pages) > 1 {
		var nav []tgbotapi.InlineKeyboardButton
		if textPage > 0 {
			nav = append(nav, tgbotapi.NewInlineKeyboardButtonData("⬅️ Prev Part", fmt.Sprintf("hadith_page:r:%s:%d:%d", colName, hadithNum, textPage-1)))
		}
		if textPage < len(pages)-1 {
			nav = append(nav, tgbotapi.NewInlineKeyboardButtonData("Next Part ➡️", fmt.Sprintf("hadith_page:r:%s:%d:%d", colName, hadithNum, textPage+1)))
		}
		if len(nav) > 0 {
			rows = append(rows, nav)
		}
	}

	shareURL := fmt.Sprintf("https://t.me/%s?start=hadith_%s_%d", h.bot.Self.UserName, colName, hadithNum)
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🎲 Another Random", "random"),
		tgbotapi.NewInlineKeyboardButtonData("🎨 Image", fmt.Sprintf("hadith_image:%s:%d", colName, hadithNum)),
		tgbotapi.NewInlineKeyboardButtonURL("📤 Share", shareURL),
	))
	h.editOrSendMessage(chatID, msgID, inlineMsgID, display, tgbotapi.NewInlineKeyboardMarkup(rows...))
}

func (h *Handler) handleHadithImageCallback(c *tgbotapi.CallbackQuery, parts []string) {
	// parts: hadith_image:collection:hadithNum
	if len(parts) < 3 {
		return
	}
	col := parts[1]
	hadithNum, _ := strconv.Atoi(parts[2])

	// Determine where to send the image
	var chatID int64
	if c.Message != nil {
		chatID = c.Message.Chat.ID
	} else {
		// Inline button: try sending to user's private chat
		chatID = c.From.ID
	}

	// Answer callback to show loading state (toast or just stop loading)
	h.bot.Request(tgbotapi.NewCallback(c.ID, "🎨 Generating image..."))

	// Fetch hadith
	hadith, _ := h.hadithService.FindHadithByNumber(col, hadithNum)
	if hadith == nil {
		h.sendMessage(chatID, "⚠️ Could not find hadith.")
		return
	}

	h.bot.Send(tgbotapi.NewChatAction(chatID, "upload_photo"))

	// Generate image
	book := h.hadithService.GetBook(col, hadith.ChapterID)
	title := "Hadith"
	if book != nil {
		title = book.Title
		// Clean up title if it has numbers like "1. Book of ..."
		if idx := strings.Index(title, ". "); idx != -1 {
			title = title[idx+2:]
		}
	}

	ref := fmt.Sprintf("[%s: %d]", services.GetCollectionDisplayName(col), hadith.HadithNumber)

	useCustomBg := false
	if chatState := h.state.GetChatState(c.From.ID); chatState != nil {
		useCustomBg = chatState.UseCustomBg
	}

	imgBytes, err := h.imageGenerator.GenerateHadithImage(title, hadith.Narrator, hadith.Arabic, hadith.English, ref, useCustomBg)
	if err != nil {
		h.log.Error("Failed to generate image: %v", err)
		h.sendMessage(chatID, "⚠️ Failed to generate image.")
		return
	}

	// Send photo
	photo := tgbotapi.NewPhoto(chatID, tgbotapi.FileBytes{
		Name:  "hadith.png",
		Bytes: imgBytes,
	})
	photo.Caption = fmt.Sprintf("Hadith #%d from %s", hadith.HadithNumber, services.GetCollectionDisplayName(col))
	h.bot.Send(photo)
}

// --- FORMATTING & UTILS ---

func (h *Handler) editOrSendMessage(chatID int64, msgID int, inlineMsgID string, text string, kb tgbotapi.InlineKeyboardMarkup) {
	if inlineMsgID != "" {
		edit := tgbotapi.EditMessageTextConfig{
			BaseEdit: tgbotapi.BaseEdit{
				InlineMessageID: inlineMsgID,
				ReplyMarkup:     &kb,
			},
			Text:      text,
			ParseMode: tgbotapi.ModeHTML,
		}
		h.bot.Send(edit)
	} else if msgID != 0 {
		edit := tgbotapi.NewEditMessageText(chatID, msgID, text)
		edit.ParseMode = tgbotapi.ModeHTML
		edit.ReplyMarkup = &kb
		h.bot.Send(edit)
	} else {
		msg := tgbotapi.NewMessage(chatID, text)
		msg.ParseMode = tgbotapi.ModeHTML
		msg.ReplyMarkup = kb
		h.bot.Send(msg)
	}
}

func (h *Handler) sendMessage(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeHTML
	h.bot.Send(msg)
}

func (h *Handler) sendMessageWithKeyboard(chatID int64, text string, kb tgbotapi.InlineKeyboardMarkup) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeHTML
	msg.ReplyMarkup = kb
	h.bot.Send(msg)
}

func (h *Handler) sendSearchResults(chatID int64, msgID int, inlineMsgID string, query string, res models.SearchResult) {
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, hd := range res.Hadiths {
		col := h.findCollectionForHadith(hd)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("📜 Hadith #%d (%s)", hd.HadithNumber, services.GetCollectionDisplayName(col)), fmt.Sprintf("hadith_search:%s:%d", col, hd.HadithNumber))))
	}

	if res.TotalPages > 1 {
		var nav []tgbotapi.InlineKeyboardButton
		if res.Page > 1 {
			nav = append(nav, tgbotapi.NewInlineKeyboardButtonData("⬅️ Prev", fmt.Sprintf("search_prev:%s:%d", query, res.Page-1)))
		}
		if res.Page < res.TotalPages {
			nav = append(nav, tgbotapi.NewInlineKeyboardButtonData("Next ➡️", fmt.Sprintf("search_next:%s:%d", query, res.Page+1)))
		}
		rows = append(rows, nav)
	}

	h.editOrSendMessage(chatID, msgID, inlineMsgID, fmt.Sprintf("🔍 <b>Results for:</b> %s", html.EscapeString(query)), tgbotapi.NewInlineKeyboardMarkup(rows...))
}

func (h *Handler) formatHadithDisplay(hdt *models.Hadith, col *models.Collection, b *models.Book) string {
	colTitle := "Unknown"
	if col != nil {
		colTitle = col.Title
	}
	bookNum := 0
	if b != nil {
		bookNum = b.BookNumber
	}
	grade := "Sahih"
	if hdt.Grade != "" {
		grade = hdt.Grade
	}

	narrator := ""
	if hdt.Narrator != "" {
		narrator = fmt.Sprintf("\n<b>Narrator:</b> %s\n", html.EscapeString(hdt.Narrator))
	}

	return fmt.Sprintf("📜 <b>Hadith</b>\n\n%s%s\n\n%s\n\n<b>Reference:</b> %s, Book %d, #%d\n<b>Grade:</b> %s",
		html.EscapeString(hdt.Arabic), narrator, html.EscapeString(hdt.English), html.EscapeString(colTitle), bookNum, hdt.HadithNumber, html.EscapeString(grade))
}

func (h *Handler) findCollectionForHadith(hadith models.Hadith) string {
	if hadith.CollectionName != "" {
		return hadith.CollectionName
	}
	// Fallback logic, though with the fix to SearchHadiths this shouldn't be needed
	for _, col := range h.hadithService.GetCollections() {
		books := h.hadithService.GetBooks(col.Name)
		for _, b := range books {
			if b.BookNumber == hadith.ChapterID {
				return col.Name
			}
		}
	}
	return "bukhari"
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func getCollectionTitle(c *models.Collection) string {
	if c == nil {
		return "Unknown"
	}
	return c.Title
}

func splitTelegramMessage(text string, maxRunes int) []string {
	if maxRunes <= 0 {
		return []string{text}
	}

	runes := []rune(text)
	if len(runes) <= maxRunes {
		return []string{text}
	}

	chunks := make([]string, 0)
	start := 0

	for start < len(runes) {
		end := start + maxRunes
		if end >= len(runes) {
			chunks = append(chunks, strings.TrimSpace(string(runes[start:])))
			break
		}

		splitAt := -1
		searchFrom := start + (maxRunes * 2 / 3)
		for i := end; i >= searchFrom && i > start; i-- {
			if runes[i-1] == '\n' {
				splitAt = i
				break
			}
		}

		if splitAt == -1 {
			splitAt = end
		}

		chunk := strings.TrimSpace(string(runes[start:splitAt]))
		if chunk != "" {
			chunks = append(chunks, chunk)
		}
		start = splitAt
	}

	return chunks
}

func escapeHTML(text string) string {
	return html.EscapeString(text)
}
