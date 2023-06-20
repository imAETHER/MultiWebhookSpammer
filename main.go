package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

const (
	ScreenMain = iota
	ScreenManual
	ScreenSpam

	maxColumns int = 4
)

var (
	// Regex for checking if the webhook's valid
	whRegex = regexp.MustCompile(`(?i)^.*(discord|discordapp)\.com\/api\/webhooks\/([\d]+)\/([a-z0-9_-]+)$`)

	// STYLING
	color      = termenv.EnvColorProfile().Color
	colBanner  = termenv.Style{}.Foreground(color("#da8f1b")).Italic().Styled
	colDefault = termenv.Style{}.Foreground(color("15")).Styled
	colInfo    = termenv.Style{}.Foreground(color("8")).Styled
	striked    = termenv.Style{}.CrossOut().Styled

	webhooks    = []Webhook{}
	spamMessage *WebhookData

	whFileExists = true
	whFileData   string

	aeBanner = colBanner("       db                             88                                   \n" +
		"      d88b                     ,d     88                                   \n" +
		"     d8'`8b                    88     88                                   \n" +
		"    d8'  `8b      ,adPPYba,  MM88MMM  88,dPPYba,    ,adPPYba,  8b,dPPYba,  \n" +
		"   d8YaaaaY8b    a8P_____88    88     88P'    \"8a  a8P_____88  88P'   \"Y8  \n" +
		"  d8\"\"\"\"\"\"\"\"8b   8PP\"\"\"\"\"\"\"    88     88       88  8PP\"\"\"\"\"\"\"  88          \n" +
		" d8'        `8b  \"8b,   ,aa    88,    88       88  \"8b,   ,aa  88          \n" +
		"d8'          `8b  `\"Ybbd8\"'    \"Y888  88       88   `\"Ybbd8\"'  88          \n\n")

	currentScreen int = ScreenMain

	// Green
	whAliveModel = lipgloss.NewStyle().
			Width(20).
			Height(5).
			Align(lipgloss.Center).
			BorderStyle(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color("#3ba55c"))
	// Red
	whDeadModel = lipgloss.NewStyle().
			Width(20).
			Height(5).
			Align(lipgloss.Center).
			BorderStyle(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color("#ab0c25"))
	// Purple-ish
	whRatelimitModel = lipgloss.NewStyle().
				Width(20).
				Height(5).
				Align(lipgloss.Center).
				BorderStyle(lipgloss.DoubleBorder()).
				BorderForeground(lipgloss.Color("#ce6bcf"))

	isSpamming = false
	shouldSpam = false
)

type WOwner struct {
	Id       string `json:"id"`
	Username string `json:"username"`
	Discrim  string `json:"discriminator"`
}

type Webhook struct {
	Name        string
	Owner       WOwner
	Alive       bool
	Url         string
	TotalSent   int
	TotalMissed int
	Ratelimit   float64
}

type WImage struct {
	URL string `json:"url,omitempty"`
}

type WFooter struct {
	IconURL string `json:"icon_url,omitempty"`
	Text    string `json:"text,omitempty"`
}

type WebhookEmbed struct {
	Color       int     `json:"color,omitempty"`
	Description string  `json:"description,omitempty"`
	Footer      WFooter `json:"footer,omitempty"`
	Image       WImage  `json:"image,omitempty"`
	Title       string  `json:"title,omitempty"`
}

type WebhookData struct {
	Content string         `json:"content"`
	Embeds  []WebhookEmbed `json:"embeds"`
}

type WebhookInfo struct {
	After float64 `json:"retry_after"`
	Name  string  `json:"name"`
	Owner WOwner  `json:"user"`
}

type PModel struct {
	choices  []string
	cursor   int
	textarea textarea.Model
	spinner  spinner.Model
}

func init() {
	// Default message, for when reading message.json fails
	spamMessage = &WebhookData{
		Content: "@everyone",
		Embeds: []WebhookEmbed{
			{
				Title:       "+ A E T H E R +",
				Description: "Get spammed lol",
				Image: WImage{
					URL: "https://cdn.discordapp.com/attachments/1036756732497637506/1055141759459532800/Fj_KV71XEAQOfDl.jpg",
				},
				Footer: WFooter{
					IconURL: "https://cdn.discordapp.com/attachments/832763964558802984/891043235915526195/IidUDunEiBNEoe.png",
					Text:    "MWS by: github.com/imAETHER",
				},
				Color: 14194190,
			},
		},
	}

	mfile, err := os.OpenFile("message.json", os.O_RDWR|os.O_CREATE, fs.ModePerm)
	if err != nil {
		fmt.Println("Failed to read message.json!")
	} else {
		defer mfile.Close()

		mb, err := io.ReadAll(mfile)
		if err != nil {
			panic(err)
		}

		if len(mb) < 10 { //why 10? idk
			newone, _ := json.MarshalIndent(spamMessage, "", " ")
			mfile.Write(newone)
		} else {
			err = json.Unmarshal(mb, spamMessage)
			if err != nil {
				fmt.Println("Failed to read unmarshal spam message data!")
			}
		}
	}

	wfile, err := os.OpenFile("webhooks.txt", os.O_RDWR, fs.ModePerm)
	if err != nil {
		whFileExists = false
		return
	}
	defer wfile.Close()

	b, err := io.ReadAll(wfile)
	if err != nil {
		fmt.Println("Failed to read webhooks.txt!")
		panic(err)
	}
	whFileData = string(b)
}

func setupOptions() PModel {
	choice2 := "Load from file (webhooks.txt)"
	if !whFileExists {
		choice2 = striked(choice2) + " Not found!"
	}

	// for ScreenManual
	txtarea := textarea.New()
	txtarea.SetHeight(11)
	txtarea.SetWidth(130)
	txtarea.CharLimit = 0
	txtarea.Placeholder = "https://discord.com/api/webhooks..."

	// for ScreenSpam
	spin := spinner.New()
	spin.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("69"))
	spin.Spinner = spinner.Points

	return PModel{
		choices:  []string{"Manual Input (slow)", choice2},
		textarea: txtarea,
		spinner:  spin,
	}
}

func (m PModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func executeWebhooks() {
	if !shouldSpam {
		return
	}
	isSpamming = true

	for i := range webhooks {
		go func(index int) {
			for {
				if !shouldSpam || len(webhooks) < index {
					return
				}

				webhook := &webhooks[index]

				if !whRegex.MatchString(webhook.Url) {
					// Webhook url was invalid from the start
					webhook.Alive = false
					return
				}

				client := &http.Client{}

				// If the webhook doesnt have a name set, fetch it
				if webhook.Name == "" {
					req, err := http.NewRequest("GET", webhook.Url, nil)
					if err != nil {
						return
					}

					req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; rv:108.0) Gecko/20100101 Firefox/108.0")

					res, err := client.Do(req)
					if err != nil {
						return
					}
					defer res.Body.Close()

					if res.StatusCode != http.StatusOK {
						webhook.Alive = false
						return
					}

					bytes, err := io.ReadAll(res.Body)
					if err != nil {
						return
					}

					var whinfo WebhookInfo
					err = json.Unmarshal(bytes, &whinfo)
					if err != nil {
						return
					}

					webhook.Name = whinfo.Name
					webhook.Owner = whinfo.Owner
				}

				postBody, err := json.Marshal(spamMessage)
				if err != nil {
					return
				}

				req, err := http.NewRequest("POST", webhook.Url, bytes.NewReader(postBody))
				if err != nil {
					return
				}

				req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; rv:108.0) Gecko/20100101 Firefox/108.0")
				req.Header.Set("Content-type", "application/json")

				res, err := client.Do(req)
				if err != nil {
					return
				}
				defer res.Body.Close()

				switch res.StatusCode {
				case http.StatusNotFound:
					webhook.Alive = false
					return
				case http.StatusTooManyRequests:
					bytes, err := io.ReadAll(res.Body)
					if err != nil {
						return
					}

					var wakawaka WebhookInfo
					err = json.Unmarshal(bytes, &wakawaka)
					if err != nil {
						return
					}

					if wakawaka.After < 1 {
						break
					}

					webhook.Ratelimit = wakawaka.After
					webhook.TotalMissed++
					time.Sleep(time.Duration(wakawaka.After+1) * time.Second)
					webhook.Ratelimit = 0
				case http.StatusOK, http.StatusNoContent:
					webhook.Ratelimit = 0
					webhook.TotalSent++
				default:
					bytes, err := io.ReadAll(res.Body)
					if err != nil {
						return
					}
					fmt.Printf("Status: %d | response: %s \n", res.StatusCode, string(bytes))
				}

				time.Sleep(44 * time.Millisecond)
			}
		}(i)
	}
}

func (m PModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Screens
		switch currentScreen {
		// Main screen
		case ScreenMain:
			shouldSpam = false
			switch msg.String() {

			case "ctrl+c":
				return m, tea.Quit

			case "up", "w":
				if m.cursor > 0 {
					m.cursor--
				}

			case "down", "s":
				if m.cursor < len(m.choices)-1 {
					m.cursor++
				}

			case "enter":
				switch m.cursor {
				case 0:
					currentScreen = ScreenManual
				case 1: // Load from file
					if !whFileExists {
						break
					}

					// Add all the webhooks
					for _, v := range strings.Split(whFileData, "\n") {
						webhooks = append(webhooks, Webhook{
							Alive:       true,
							Url:         strings.Trim(v, "\n\t\r"),
							TotalSent:   0,
							TotalMissed: 0,
						})
					}

					currentScreen = ScreenSpam
				}
			}
			// Manual webhook input
		case ScreenManual:
			shouldSpam = false
			switch msg.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "ctrl+s":
				if m.textarea.Focused() {
					m.textarea.Blur()
				}

				for _, v := range strings.Split(m.textarea.Value(), "\n") {
					webhooks = append(webhooks, Webhook{
						Alive:       true,
						Url:         v,
						TotalSent:   0,
						TotalMissed: 0,
					})
				}

				currentScreen = ScreenSpam
			case "esc":
				currentScreen = ScreenMain
				m.textarea.SetValue("")
			default:
				if !m.textarea.Focused() {
					cmd = m.textarea.Focus()
					cmds = append(cmds, cmd)
				}
			}

			m.textarea, cmd = m.textarea.Update(msg)

			cmds = append(cmds, cmd)
			return m, tea.Batch(cmds...)
			// Actual spam screen logic, not much
		case ScreenSpam:
			switch msg.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "esc":
				shouldSpam = false
				isSpamming = false

				webhooks = []Webhook{}

				currentScreen = ScreenMain
				return m, tea.Batch(cmds...)
			}
		}
	case spinner.TickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m PModel) View() string {
	var body string

	switch currentScreen {
	case ScreenMain:
		body := aeBanner

		// Iterate over our choices
		for i, choice := range m.choices {
			cursor := " "
			if m.cursor == i {
				cursor = ">"
			}

			//TODO: find a better way of reseting styling
			body += colDefault(fmt.Sprintf(termenv.CSI+termenv.ResetSeq+"m"+" [%s] %s\n", cursor, choice))
		}

		body += "\n" + colInfo("(i) Press CTRL + C to exit.")
		return body
	case ScreenManual:
		body := colBanner("Paste your webhooks here, press ENTER for a new line")
		body += "\n\n"
		body += m.textarea.View()

		body += "\n\n" + colInfo("(i) Press CTRL+S to save & spam, or ESCAPE to discard.")
		return body
	case ScreenSpam:
		shouldSpam = true
		if !isSpamming {
			executeWebhooks()
		}

		// I feel like I could do a simple calculation to avoid having to grow it in the loop
		displayRows := make([][]string, 1)

		aliveHooks := 0
		deadHooks := 0
		limitedHooks := 0

		totalRows := 0
		tempIndex := 0
		for _, wh := range webhooks {
			if wh.Alive {
				aliveHooks++
				if wh.Ratelimit > 0 {
					displayRows[totalRows] = append(displayRows[totalRows], whRatelimitModel.Render(fmt.Sprintf("%s\n\n%s\n\nOwner: %s#%s\n\nSent: %d\nTimeout: %.2f", m.spinner.View(), wh.Name, wh.Owner.Username, wh.Owner.Discrim, wh.TotalSent, wh.Ratelimit)))
					limitedHooks++
				} else {
					displayRows[totalRows] = append(displayRows[totalRows], whAliveModel.Render(fmt.Sprintf("%s\n\n%s\n\nOwner: %s#%s\n\nSent: %d", m.spinner.View(), wh.Name, wh.Owner.Username, wh.Owner.Discrim, wh.TotalSent)))
				}
			} else {
				deadHooks++
				displayRows[totalRows] = append(displayRows[totalRows], whDeadModel.Render(fmt.Sprintf("%s\n\n%s\n\nOwner: %s#%s\n\nSent: %d", "DELETED", wh.Name, wh.Owner.Username, wh.Owner.Discrim, wh.TotalSent)))
			}

			if tempIndex >= maxColumns {
				displayRows = append(displayRows, make([]string, maxColumns))
				tempIndex = 0
				totalRows++
				continue
			}
			tempIndex++
		}
		body := colBanner(fmt.Sprintf("[!] Spamming | %d total | %d alive | %d dead/deleted | %d ratelimited\n\n", len(webhooks), aliveHooks, deadHooks, limitedHooks))

		for _, row := range displayRows {
			body += lipgloss.JoinHorizontal(lipgloss.Top, row...)
			body += "\n"
		}

		body += "\n\n" + colInfo("(i) Press ESCAPE to exit.")
		return body
	}

	// Send the UI for rendering
	return body
}

func main() {
	var err error
	restoreConsole, err := termenv.EnableVirtualTerminalProcessing(termenv.DefaultOutput())
	if err != nil {
		panic(err)
	}
	defer restoreConsole()

	termenv.DefaultOutput().SetWindowTitle("Aether's MultiWebhook Spammer v2.2")

	p := tea.NewProgram(setupOptions(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
