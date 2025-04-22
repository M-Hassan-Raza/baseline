package main

// --- Imports ---
// Standard library
import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"sort" // <-- Added import for sort package
	"strconv"
	"strings"
	"sync"
	"time"

	// External libraries
	"github.com/gdamore/tcell/v2" // Terminal cell manipulation (used by tview)
	"github.com/joho/godotenv"    // For .env file loading
	"github.com/rivo/tview"       // TUI library

	// gopsutil modules (need to 'go get' these)
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
)

// --- Constants & Configuration ---
const (
	appName         = "Baseline"
	refreshInterval = 2 * time.Second // How often to refresh data
	historyLimit    = 60              // Max data points for history
)

// Theme definition (using tcell colors)
type Theme struct {
	Main   tcell.Color
	Dim    tcell.Color
	Bright tcell.Color
}

var themes = map[string]Theme{
	"amber": {
		Main:   tcell.NewHexColor(0xFFBF00), // #FFBF00
		Dim:    tcell.NewHexColor(0xCC9900), // #CC9900
		Bright: tcell.NewHexColor(0xFFDF00), // #FFDF00
	},
	"green": {
		Main:   tcell.NewHexColor(0x00FF00), // #00FF00
		Dim:    tcell.NewHexColor(0x009900), // #009900
		Bright: tcell.NewHexColor(0xCCFFCC), // #CCFFCC
	},
	"blue": {
		Main:   tcell.NewHexColor(0x00BFFF), // #00BFFF
		Dim:    tcell.NewHexColor(0x0099CC), // #0099CC
		Bright: tcell.NewHexColor(0x99CCFF), // #99CCFF
	},
}

// --- Data Structures ---

type TodoItem struct {
	Text     string `json:"text"`
	Done     bool   `json:"done"`
	Priority string `json:"priority"` // "low", "medium", "high"
}

type Notification struct {
	Message string
	Type    string // "info", "error", "success"
	Time    time.Time
}

type SystemHistory struct {
	CPU        []float64 `json:"cpu"`
	Memory     []float64 `json:"memory"`
	Timestamps []string  `json:"timestamps"`
	NetworkIn  []uint64  `json:"network_in"`
	NetworkOut []uint64  `json:"network_out"`
}

type WeatherInfo struct {
	Location    string
	TempC       float64
	Condition   string
	Humidity    int
	WindKph     float64
	Error       string
	LastUpdated time.Time
}

// --- Baseline Application Struct ---

type Baseline struct {
	app *tview.Application // The TUI application

	// UI Components
	layout       *tview.Flex
	header       *tview.TextView
	systemPanel  *tview.TextView
	weatherPanel *tview.TextView
	timePanel    *tview.TextView
	todoPanel    *tview.TextView
	footer       *tview.TextView // For notifications
	cmdInput     *tview.InputField // For command input

	// State
	mu              sync.RWMutex // Mutex for thread-safe access to shared state
	configDir       string
	todoItems       []TodoItem
	notifications   []Notification
	systemHistory   SystemHistory
	weatherInfo     WeatherInfo
	lastNetIO       net.IOCountersStat
	lastNetTime     time.Time
	currentFocus    string // "dashboard", "command", "todoInput" (maybe later)
	commandHistory  []string
	theme           Theme
	weatherAPIKey   string
	weatherLocation string
	cpuCoreCount    int
}

// --- Constructor ---

func NewBaseline() *Baseline {
	// Load .env - ignore error if it doesn't exist
	_ = godotenv.Load()

	// Determine config directory (~/.baseline)
	usr, err := user.Current()
	var configDir string
	if err != nil {
		log.Printf("Warning: Could not get user home directory: %v. Using current dir.", err)
		configDir = ".baseline"
	} else {
		configDir = filepath.Join(usr.HomeDir, ".baseline")
	}
	// Create config dir if it doesn't exist
	_ = os.MkdirAll(configDir, 0750)

	// Get CPU core count
	cpuCount, err := cpu.Counts(true) // Logical cores
	if err != nil || cpuCount == 0 {
		log.Printf("Warning: Could not get CPU count: %v. Defaulting to 1.", err)
		cpuCount = 1
	}

	// Get theme from env or default
	themeName := os.Getenv("THEME")
	if themeName == "" {
		themeName = "amber"
	}
	selectedTheme, ok := themes[themeName]
	if !ok {
		log.Printf("Warning: Theme '%s' not found. Defaulting to amber.", themeName)
		selectedTheme = themes["amber"]
	}

	b := &Baseline{
		app:             tview.NewApplication(),
		configDir:       configDir,
		currentFocus:    "dashboard",
		theme:           selectedTheme,
		weatherAPIKey:   os.Getenv("WEATHER_API_KEY"),
		weatherLocation: os.Getenv("WEATHER_LOCATION"),
		cpuCoreCount:    cpuCount,
	}

	if b.weatherLocation == "" {
		b.weatherLocation = "Lahore" // Default location
	}
	if b.weatherAPIKey == "YOUR_API_KEY" || b.weatherAPIKey == "" {
		b.weatherAPIKey = "" // Treat as unset
		b.addNotification("Weather API key not set. Using sample data.", "info")
	}

	b.loadTodos()
	b.loadSystemHistory()
	// Get initial network stats
	ioc, err := net.IOCounters(false) // Get aggregate counters
	if err == nil && len(ioc) > 0 {
		b.lastNetIO = ioc[0]
		b.lastNetTime = time.Now()
	}

	return b
}

// --- File I/O ---

func (b *Baseline) loadTodos() {
	b.mu.Lock()
	defer b.mu.Unlock()

	filePath := filepath.Join(b.configDir, "todos.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// Default todos if file doesn't exist
			b.todoItems = []TodoItem{
				{Text: "Review project documentation (Go)", Done: false, Priority: "medium"},
				{Text: "Debug terminal interface (Go)", Done: true, Priority: "high"},
				{Text: "Implement weather module (Go)", Done: false, Priority: "medium"},
				{Text: "Optimize system performance (Go)", Done: false, Priority: "low"},
			}
		} else {
			b.addNotification(fmt.Sprintf("Error loading todos: %v", err), "error")
			b.todoItems = []TodoItem{} // Ensure it's initialized
		}
		return
	}

	err = json.Unmarshal(data, &b.todoItems)
	if err != nil {
		b.addNotification(fmt.Sprintf("Error parsing todos.json: %v", err), "error")
		b.todoItems = []TodoItem{} // Ensure it's initialized on error
	}
}

func (b *Baseline) saveTodos() {
	// Called from within locked sections or needs its own lock if called externally
	filePath := filepath.Join(b.configDir, "todos.json")
	data, err := json.MarshalIndent(b.todoItems, "", "  ") // Pretty print JSON
	if err != nil {
		b.addNotification(fmt.Sprintf("Error marshalling todos: %v", err), "error")
		return
	}

	err = os.WriteFile(filePath, data, 0640) // Write with permissions
	if err != nil {
		b.addNotification(fmt.Sprintf("Error saving todos: %v", err), "error")
	}
}

func (b *Baseline) loadSystemHistory() {
	b.mu.Lock()
	defer b.mu.Unlock()

	filePath := filepath.Join(b.configDir, "system_history.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		if !os.IsNotExist(err) {
			b.addNotification(fmt.Sprintf("Error loading history: %v", err), "error")
		}
		// Initialize if error or not exists
		b.systemHistory = SystemHistory{
			CPU:        []float64{},
			Memory:     []float64{},
			Timestamps: []string{},
			NetworkIn:  []uint64{},
			NetworkOut: []uint64{},
		}
		return
	}

	err = json.Unmarshal(data, &b.systemHistory)
	if err != nil {
		b.addNotification(fmt.Sprintf("Error parsing history.json: %v", err), "error")
		// Initialize on error
		b.systemHistory = SystemHistory{
			CPU:        []float64{},
			Memory:     []float64{},
			Timestamps: []string{},
			NetworkIn:  []uint64{},
			NetworkOut: []uint64{},
		}
	}
}

func (b *Baseline) saveSystemHistory() {
	// Called from within locked sections
	filePath := filepath.Join(b.configDir, "system_history.json")

	// Trim history if needed
	if len(b.systemHistory.CPU) > historyLimit {
		b.systemHistory.CPU = b.systemHistory.CPU[len(b.systemHistory.CPU)-historyLimit:]
		b.systemHistory.Memory = b.systemHistory.Memory[len(b.systemHistory.Memory)-historyLimit:]
		b.systemHistory.Timestamps = b.systemHistory.Timestamps[len(b.systemHistory.Timestamps)-historyLimit:]
		b.systemHistory.NetworkIn = b.systemHistory.NetworkIn[len(b.systemHistory.NetworkIn)-historyLimit:]
		b.systemHistory.NetworkOut = b.systemHistory.NetworkOut[len(b.systemHistory.NetworkOut)-historyLimit:]
	}

	data, err := json.MarshalIndent(b.systemHistory, "", "  ")
	if err != nil {
		b.addNotification(fmt.Sprintf("Error marshalling history: %v", err), "error")
		return
	}

	err = os.WriteFile(filePath, data, 0640)
	if err != nil {
		b.addNotification(fmt.Sprintf("Error saving history: %v", err), "error")
	}
}

// --- UI Setup ---

func (b *Baseline) setupLayout() {
	// --- Fix Start ---
	// Initialize TextViews first, then set properties
	b.header = tview.NewTextView()
	b.header.SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true)

	b.systemPanel = tview.NewTextView()
	b.systemPanel.SetDynamicColors(true).
		SetScrollable(true).
		SetBorder(true). // Returns *Box
		SetTitle(" System Status ") // Returns *Box

	b.weatherPanel = tview.NewTextView()
	b.weatherPanel.SetDynamicColors(true).
		SetScrollable(true).
		SetBorder(true).
		SetTitle(" Weather Report ")

	b.timePanel = tview.NewTextView()
	b.timePanel.SetDynamicColors(true).
		SetScrollable(true).
		SetBorder(true).
		SetTitle(" Time & Calendar ")

	b.todoPanel = tview.NewTextView()
	b.todoPanel.SetDynamicColors(true).
		SetScrollable(true).
		SetBorder(true).
		SetTitle(" Task List ")
	// --- Fix End ---

	b.footer = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(false) // Usually single line

	b.cmdInput = tview.NewInputField().
		SetLabel("> ").
		SetLabelColor(b.theme.Bright).
		SetFieldBackgroundColor(tcell.ColorBlack). // Match background
		SetFieldTextColor(b.theme.Main)

	// Command Input Done handler
	b.cmdInput.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			cmd := b.cmdInput.GetText()
			b.processCommand(cmd)
			// Switch focus back to main layout after command
			b.app.SetFocus(b.layout)
			b.currentFocus = "dashboard"
			b.updateFooter() // Update footer to show notifications again
		} else if key == tcell.KeyEscape {
			b.cmdInput.SetText("")
			b.app.SetFocus(b.layout)
			b.currentFocus = "dashboard"
			b.updateFooter()
		}
		// Potentially add history navigation (Up/Down arrows) here later
	})

	// Layout structure (similar to Python's Rich layout)
	leftPanel := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(b.systemPanel, 0, 1, false). // Proportions adjust automatically
		AddItem(b.weatherPanel, 0, 1, false)

	rightPanel := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(b.timePanel, 0, 1, false).
		AddItem(b.todoPanel, 0, 2, false) // Todo gets twice the height ratio

	mainContent := tview.NewFlex().
		AddItem(leftPanel, 0, 1, false). // Left takes half width
		AddItem(rightPanel, 0, 1, false) // Right takes half width

	// Main layout with Header, Main Content, Footer
	b.layout = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(b.header, 3, 0, false).       // Header fixed height
		AddItem(mainContent, 0, 1, true).      // Main content takes remaining space, gets focus
		AddItem(b.footer, 1, 0, false).        // Footer fixed height (for notifications)
		AddItem(b.cmdInput, 1, 0, false) // Command input, same space as footer, initially hidden

	// Initially hide command input, show footer
	b.layout.ResizeItem(b.footer, 1, 0)
	b.layout.ResizeItem(b.cmdInput, 0, 0)

	// Apply theme colors
	b.applyTheme()
}

func (b *Baseline) applyTheme() {
	// --- Fix Start ---
	// Removed unused mainColorStr, dimColorStr, brightColorStr declarations
	// --- Fix End ---

	// Set border colors
	b.systemPanel.SetBorderColor(b.theme.Main)
	b.weatherPanel.SetBorderColor(b.theme.Main)
	b.timePanel.SetBorderColor(b.theme.Main)
	b.todoPanel.SetBorderColor(b.theme.Main)
	// Footer/CmdInput don't have borders in this setup

	// Set title colors (usually same as border)
	b.systemPanel.SetTitleColor(b.theme.Main)
	b.weatherPanel.SetTitleColor(b.theme.Main)
	b.timePanel.SetTitleColor(b.theme.Main)
	b.todoPanel.SetTitleColor(b.theme.Main)

	// Set default text colors (can be overridden with tags)
	b.header.SetTextColor(b.theme.Main)
	b.systemPanel.SetTextColor(b.theme.Main)
	b.weatherPanel.SetTextColor(b.theme.Main)
	b.timePanel.SetTextColor(b.theme.Main)
	b.todoPanel.SetTextColor(b.theme.Main)
	b.footer.SetTextColor(b.theme.Dim) // Default footer text is dim

	// Command input styling
	b.cmdInput.SetLabelColor(b.theme.Bright)
	b.cmdInput.SetFieldTextColor(b.theme.Main)

	// Force redraw with new colors
	b.updateHeader()
	b.updateSystemInfo()
	b.updateWeather()
	b.updateTime()
	b.updateTodos()
	b.updateFooter()
}

// --- UI Update Methods ---

// Helper to get color string for tview tags
func colorTag(color tcell.Color) string {
	// Use Hex() which returns int32, format as 6-digit hex
	return fmt.Sprintf("[#%06x]", color.Hex())
}

func (b *Baseline) updateHeader() {
	b.mu.RLock()
	defer b.mu.RUnlock()

	now := time.Now()
	hostName, _ := os.Hostname()
	userName := "user"
	currentUser, err := user.Current()
	if err == nil {
		userName = currentUser.Username
	}

	mainColor := colorTag(b.theme.Main)
	dimColor := colorTag(b.theme.Dim)

	headerText := fmt.Sprintf("%s%s%s[-:-:-]\n", mainColor, "[::b]", appName) // Bold main title
	subHeaderText := fmt.Sprintf("%s[Session: %s] [Terminal: %s@%s][-:-:-]",
		dimColor,
		now.Format("2006-01-02"),
		userName,
		hostName,
	)

	// Use QueueUpdateDraw for thread safety if called from goroutine,
	// but direct update is fine if called only from main thread or setup
	b.header.SetText(headerText + subHeaderText)
}

func (b *Baseline) updateSystemInfo() {
	b.mu.Lock() // Lock for writing history
	defer b.mu.Unlock()

	// --- Gather Data ---
	var cpuPercent float64
	cpuPercents, err := cpu.Percent(0, false) // Get overall CPU percentage
	if err == nil && len(cpuPercents) > 0 {
		cpuPercent = cpuPercents[0]
	}

	memInfo, err := mem.VirtualMemory()
	memPercent := 0.0
	if err == nil {
		memPercent = memInfo.UsedPercent
	}

	diskInfo, err := disk.Usage("/")
	diskPercent := 0.0
	if err == nil {
		diskPercent = diskInfo.UsedPercent
	}

	hostInfo, _ := host.Info()
	uptime := time.Duration(0)
	bootTime := time.Time{}
	if hostInfo != nil {
		uptime = time.Duration(hostInfo.Uptime) * time.Second
		bootTime = time.Unix(int64(hostInfo.BootTime), 0)
	}


	// Network I/O Calculation
	var rxRate, txRate float64
	currentNetIO, err := net.IOCounters(false) // Aggregate
	currentTime := time.Now()
	if err == nil && len(currentNetIO) > 0 {
		timeDiff := currentTime.Sub(b.lastNetTime).Seconds()
		if timeDiff > 0 && b.lastNetTime.Unix() > 0 { // Ensure lastNetTime is initialized
			rxRate = float64(currentNetIO[0].BytesRecv-b.lastNetIO.BytesRecv) / timeDiff / 1024 // KB/s
			txRate = float64(currentNetIO[0].BytesSent-b.lastNetIO.BytesSent) / timeDiff / 1024 // KB/s
		}
		b.lastNetIO = currentNetIO[0]
		b.lastNetTime = currentTime
	}

	// Top Processes
	procs, err := process.Processes()
	processInfos := []struct {
		Name string
		CPU  float64
	}{}
	if err == nil {
		for _, p := range procs {
			name, _ := p.Name()
			// Get CPU % since last call, requires a short sleep or interval
			// For simplicity here, we might get 0 often if called too rapidly.
			// A better approach involves storing last CPU times.
			// Using p.CPUPercent() directly might be sufficient for a snapshot.
			cpuP, _ := p.CPUPercent()
			if cpuP > 0.1 { // Only consider processes with some CPU usage
				processInfos = append(processInfos, struct {
					Name string
					CPU  float64
				}{Name: name, CPU: cpuP / float64(b.cpuCoreCount)}) // Normalize
			}
		}
		// Sort by CPU descending
		sort.Slice(processInfos, func(i, j int) bool {
			return processInfos[i].CPU > processInfos[j].CPU
		})
	}

	// --- Update History ---
	nowStr := time.Now().Format("15:04:05")
	b.systemHistory.CPU = append(b.systemHistory.CPU, cpuPercent)
	b.systemHistory.Memory = append(b.systemHistory.Memory, memPercent)
	b.systemHistory.Timestamps = append(b.systemHistory.Timestamps, nowStr)
	if len(currentNetIO) > 0 {
		b.systemHistory.NetworkIn = append(b.systemHistory.NetworkIn, currentNetIO[0].BytesRecv)
		b.systemHistory.NetworkOut = append(b.systemHistory.NetworkOut, currentNetIO[0].BytesSent)
	}
	b.saveSystemHistory() // Save (includes trimming)

	// --- Format Output ---
	mainC := colorTag(b.theme.Main)
	dimC := colorTag(b.theme.Dim)
	brightC := colorTag(b.theme.Bright)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%sSYSTEM STATUS[-:-:-]\n", brightC+"[::b]")) // Bold title
	if hostInfo != nil {
		sb.WriteString(fmt.Sprintf("%sHost: %s[-:-:-]\n", mainC, hostInfo.Hostname))
		sb.WriteString(fmt.Sprintf("%sOS: %s %s (%s)[-:-:-]\n", mainC, hostInfo.OS, hostInfo.Platform, hostInfo.PlatformVersion))
		sb.WriteString(fmt.Sprintf("%sUptime: %s[-:-:-]\n", mainC, formatDuration(uptime)))
		sb.WriteString(fmt.Sprintf("%sBoot: %s[-:-:-]\n", dimC, bootTime.Format("2006-01-02 15:04")))
	} else {
		sb.WriteString(fmt.Sprintf("%sHost/OS Info: Unavailable[-:-:-]\n", dimC))
	}


	sb.WriteString(fmt.Sprintf("\n%sCPU: %s %s %.1f%%[-:-:-]\n", mainC, createBar(cpuPercent, 15, b.theme), brightC, cpuPercent))
	sb.WriteString(fmt.Sprintf("%sMEM: %s %s %.1f%%[-:-:-]\n", mainC, createBar(memPercent, 15, b.theme), brightC, memPercent))
	sb.WriteString(fmt.Sprintf("%sDSK: %s %s %.1f%%[-:-:-]\n", mainC, createBar(diskPercent, 15, b.theme), brightC, diskPercent))

	if err == nil && len(currentNetIO) > 0 {
		sb.WriteString(fmt.Sprintf("%sNET: %s↓ %.1f KB/s ↑ %.1f KB/s[-:-:-]\n", mainC, dimC, rxRate, txRate))
	} else {
		sb.WriteString(fmt.Sprintf("%sNET: %sUnavailable[-:-:-]\n", mainC, dimC))
	}

	// Add Load Average (example of adding more info)
	loadAvg, err := load.Avg()
	if err == nil {
		sb.WriteString(fmt.Sprintf("%sLOAD: %s%.2f %.2f %.2f[-:-:-]\n", mainC, dimC, loadAvg.Load1, loadAvg.Load5, loadAvg.Load15))
	}

	sb.WriteString(fmt.Sprintf("\n%sTOP PROCESSES:[-:-:-]\n", mainC))
	limit := 3
	if len(processInfos) < limit {
		limit = len(processInfos)
	}
	for i := 0; i < limit; i++ {
		proc := processInfos[i]
		// Truncate name if too long
		name := proc.Name
		maxLen := 15
		if len(name) > maxLen {
			// Use rune count for potentially multi-byte chars
			nameRunes := []rune(name)
			if len(nameRunes) > maxLen {
				name = string(nameRunes[:maxLen-1]) + "…" // Ellipsis
			}
		}
		sb.WriteString(fmt.Sprintf("%s%-*s %sCPU: %.1f%%[-:-:-]\n", dimC, maxLen, name, mainC, proc.CPU))
	}
	if len(processInfos) == 0 {
		sb.WriteString(fmt.Sprintf("%s(No active processes found)[-:-:-]\n", dimC))
	}

	// Update the TextView
	// Use QueueUpdateDraw to ensure thread safety when updating UI from goroutine
	b.app.QueueUpdateDraw(func() {
		b.systemPanel.SetText(sb.String())
	})
}

// Helper to create text progress bar
func createBar(percentage float64, width int, theme Theme) string {
	if percentage < 0 {
		percentage = 0
	}
	if percentage > 100 {
		percentage = 100
	}
	filledWidth := int(math.Round(float64(width) * percentage / 100.0))
	emptyWidth := width - filledWidth
	if filledWidth < 0 {
		filledWidth = 0
	}
	if emptyWidth < 0 {
		emptyWidth = 0
	}

	barColor := colorTag(theme.Bright)
	emptyColor := colorTag(theme.Dim)

	return fmt.Sprintf("%s%s%s%s[-:-:-]", barColor, strings.Repeat("█", filledWidth), emptyColor, strings.Repeat("░", emptyWidth))
}

// Helper to format duration nicely
func formatDuration(d time.Duration) string {
	if d < 0 {
		return "N/A" // Handle potential negative uptime if system clock changes
	}
	d = d.Round(time.Minute)
	days := int(d.Hours()) / 24
	hrs := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60


	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hrs, mins)
	}
	if hrs > 0 {
		return fmt.Sprintf("%dh %dm", hrs, mins)
	}
	return fmt.Sprintf("%dm", mins)
}

func (b *Baseline) fetchWeather() {
	b.mu.Lock() // Lock for writing weatherInfo
	// Use a temporary variable to store fetched info
	var fetchedInfo WeatherInfo
	location := b.weatherLocation // Read location while locked
	apiKey := b.weatherAPIKey     // Read API key while locked
	b.mu.Unlock()                 // Unlock before network call

	fetchedInfo.Location = location // Set location initially
	fetchedInfo.LastUpdated = time.Now() // Update time regardless of success

	if apiKey == "" {
		// Use sample data if no API key
		fetchedInfo.TempC = 22.0
		fetchedInfo.Condition = "Partly Cloudy (Sample)"
		fetchedInfo.Humidity = 65
		fetchedInfo.WindKph = 8.0
		fetchedInfo.Error = "API Key not set"
	} else {
		url := fmt.Sprintf("https://api.weatherapi.com/v1/current.json?key=%s&q=%s", apiKey, location)
		// Set a timeout for the HTTP client
		client := http.Client{Timeout: 10 * time.Second}
		resp, err := client.Get(url)

		if err != nil {
			fetchedInfo.Error = fmt.Sprintf("HTTP error: %v", err)
		} else {
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				// Try to read error message from body
				var errResp struct {
					Error struct {
						Message string `json:"message"`
					} `json:"error"`
				}
				if json.NewDecoder(resp.Body).Decode(&errResp) == nil && errResp.Error.Message != "" {
					fetchedInfo.Error = fmt.Sprintf("API error: %s (%d)", errResp.Error.Message, resp.StatusCode)
				} else {
					fetchedInfo.Error = fmt.Sprintf("API error: Status %d", resp.StatusCode)
				}
			} else {
				var data struct {
					Location struct {
						Name string `json:"name"`
					} `json:"location"`
					Current struct {
						TempC     float64 `json:"temp_c"`
						Condition struct {
							Text string `json:"text"`
						} `json:"condition"`
						Humidity int     `json:"humidity"`
						WindKph  float64 `json:"wind_kph"`
					} `json:"current"`
				}

				if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
					fetchedInfo.Error = fmt.Sprintf("JSON parse error: %v", err)
				} else {
					// Update weather info successfully
					fetchedInfo.Location = data.Location.Name // Use name from API response
					fetchedInfo.TempC = data.Current.TempC
					fetchedInfo.Condition = data.Current.Condition.Text
					fetchedInfo.Humidity = data.Current.Humidity
					fetchedInfo.WindKph = data.Current.WindKph
					fetchedInfo.Error = "" // Clear previous error
				}
			}
		}
	}

	// Lock again to update the shared state
	b.mu.Lock()
	b.weatherInfo = fetchedInfo
	b.mu.Unlock()

	// Trigger UI update
	b.updateWeather()
}


func (b *Baseline) updateWeather() {
	b.mu.RLock() // Read lock for weatherInfo
	// Copy needed data under lock
	info := b.weatherInfo
	apiKeySet := b.weatherAPIKey != ""
	location := b.weatherLocation // Use the configured location for display if error
	b.mu.RUnlock()

	mainC := colorTag(b.theme.Main)
	dimC := colorTag(b.theme.Dim)
	brightC := colorTag(b.theme.Bright)
	errorC := "[red]" // Standard red for errors

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%sWEATHER REPORT[-:-:-]\n", brightC+"[::b]"))

	if info.Error != "" {
		sb.WriteString(fmt.Sprintf("%sLocation: %s[-:-:-]\n", mainC, location)) // Show configured location on error
		sb.WriteString(fmt.Sprintf("%sStatus: %s%s[-:-:-]\n", mainC, errorC, info.Error))
		if !apiKeySet {
			sb.WriteString(fmt.Sprintf("%s\nSet WEATHER_API_KEY in .env file[-:-:-]\n", dimC))
			// Sample ASCII art
			sb.WriteString(fmt.Sprintf("\n%s    \\  /[-:-:-]\n", brightC))
			sb.WriteString(fmt.Sprintf("%s  _ /\"\".-.    [-:-:-]\n", brightC))
			sb.WriteString(fmt.Sprintf("%s    \\_(   ).  [-:-:-]\n", brightC))
			sb.WriteString(fmt.Sprintf("%s    /(___(__)  [-:-:-]\n", brightC))
		}
	} else {
		sb.WriteString(fmt.Sprintf("%sLocation: %s[-:-:-]\n", mainC, info.Location)) // Show location from API
		sb.WriteString(fmt.Sprintf("%sTemperature: %.1f°C[-:-:-]\n", mainC, info.TempC))
		sb.WriteString(fmt.Sprintf("%sCondition: %s[-:-:-]\n", mainC, info.Condition))
		sb.WriteString(fmt.Sprintf("%sHumidity: %d%%[-:-:-]\n", dimC, info.Humidity))
		sb.WriteString(fmt.Sprintf("%sWind: %.1f km/h[-:-:-]\n", dimC, info.WindKph))
	}

	// Static Forecast Example
	sb.WriteString(fmt.Sprintf("\n%sFORECAST (Sample):[-:-:-]\n", mainC))
	hours := []string{"06:00", "12:00", "18:00", "00:00"}
	temps := []string{"18°C", "22°C", "20°C", "16°C"}
	for i, hour := range hours {
		sb.WriteString(fmt.Sprintf("%s%s: %s[-:-:-]\n", dimC, hour, temps[i]))
	}

	sb.WriteString(fmt.Sprintf("\n%sLast updated: %s[-:-:-]", dimC, info.LastUpdated.Format("15:04:05")))

	// Update the TextView
	b.app.QueueUpdateDraw(func() {
		b.weatherPanel.SetText(sb.String())
	})
}

func (b *Baseline) updateTime() {
	// No locking needed as time.Now() is safe and we don't access shared state directly here
	now := time.Now()
	mainC := colorTag(b.theme.Main)
	dimC := colorTag(b.theme.Dim)
	brightC := colorTag(b.theme.Bright)

	var sb strings.Builder

	// Current Time and Date
	sb.WriteString(fmt.Sprintf("%s%s%s[-:-:-]\n", brightC, "[::b]", now.Format("15:04:05"))) // Bold time
	sb.WriteString(fmt.Sprintf("%s%s[-:-:-]\n\n", mainC, now.Format("Monday, January 02, 2006")))

	// Calendar
	sb.WriteString(fmt.Sprintf("%s     CALENDAR     [-:-:-]\n", mainC))
	sb.WriteString(fmt.Sprintf("%sMo Tu We Th Fr Sa Su[-:-:-]\n", dimC))

	year, month, day := now.Date()
	firstOfMonth := time.Date(year, month, 1, 0, 0, 0, 0, now.Location())
	lastOfMonth := firstOfMonth.AddDate(0, 1, -1)

	// Calculate weekday of the 1st (Monday=0, Sunday=6)
	// Go's Weekday() is Sunday=0, Saturday=6. Adjust.
	startDay := int(firstOfMonth.Weekday()+6)%7 // Monday = 0
	daysInMonth := lastOfMonth.Day()

	currentDay := 1
	for week := 0; currentDay <= daysInMonth; week++ {
		var weekStr strings.Builder
		isCurrentWeek := false
		for wd := 0; wd < 7; wd++ {
			if week == 0 && wd < startDay {
				weekStr.WriteString("   ") // Padding before 1st day
			} else if currentDay <= daysInMonth {
				dayStr := strconv.Itoa(currentDay)
				if currentDay == day {
					weekStr.WriteString(fmt.Sprintf("%s%2s*[-:-:-]", brightC, dayStr)) // Highlight current day
					isCurrentWeek = true
				} else {
					weekStr.WriteString(fmt.Sprintf("%2s ", dayStr))
				}
				currentDay++
			} else {
				weekStr.WriteString("   ") // Padding after last day
			}
		}
		weekColor := dimC
		if isCurrentWeek {
			weekColor = mainC // Highlight current week's row slightly
		}
		// Ensure color tag applies to the whole line
		sb.WriteString(fmt.Sprintf("%s%s[-:-:-]\n", weekColor, weekStr.String()))
	}

	// Static Upcoming Events Example
	sb.WriteString(fmt.Sprintf("\n%sUPCOMING (Sample):[-:-:-]\n", mainC))
	events := []struct{ Time, Name string }{
		{"14:00", "Team Meeting"},
		{"16:30", "Project Review"},
		{"Tomorrow", "Deadline: Report"},
	}
	for _, event := range events {
		sb.WriteString(fmt.Sprintf("%s%s: %s%s[-:-:-]\n", dimC, event.Time, mainC, event.Name))
	}

	// Update the TextView
	b.app.QueueUpdateDraw(func() {
		b.timePanel.SetText(sb.String())
	})
}

func (b *Baseline) updateTodos() {
	b.mu.Lock() // Lock for sorting and reading/writing todos
	defer b.mu.Unlock()

	// Sort todos: High > Medium > Low, then by original order.
	priorityMap := map[string]int{"high": 0, "medium": 1, "low": 2}
	sort.SliceStable(b.todoItems, func(i, j int) bool {
		p1, ok1 := priorityMap[strings.ToLower(b.todoItems[i].Priority)]
		if !ok1 {
			p1 = 1 // Default to medium if invalid
		}
		p2, ok2 := priorityMap[strings.ToLower(b.todoItems[j].Priority)]
		if !ok2 {
			p2 = 1 // Default to medium if invalid
		}
		return p1 < p2
	})

	mainC := colorTag(b.theme.Main)
	dimC := colorTag(b.theme.Dim)
	brightC := colorTag(b.theme.Bright)

	var sb strings.Builder

	// TODO: Add input mode display if implemented later

	for i, item := range b.todoItems {
		var priorityChar string
		var priorityColor string
		switch strings.ToLower(item.Priority) {
		case "high":
			priorityChar = "!"
			priorityColor = brightC
		case "low":
			priorityChar = "-"
			priorityColor = dimC
		default: // medium
			priorityChar = "o"
			priorityColor = mainC
		}

		status := "[ ]"
		statusColor := mainC
		textColor := mainC
		if item.Done {
			status = "[X]"
			statusColor = brightC
			textColor = dimC // Dim completed tasks
		}

		// Escape brackets in the task text itself to avoid tview tag parsing issues
		escapedText := strings.ReplaceAll(item.Text, "[", "[[")
		escapedText = strings.ReplaceAll(escapedText, "]", "]]")


		sb.WriteString(fmt.Sprintf("%s%2d %s[%s] %s%s %s%s[-:-:-]\n",
			dimC, i+1, // Index
			priorityColor, priorityChar, // Priority
			statusColor, status, // Status
			textColor, escapedText, // Text (escaped)
		))
	}

	// Help text
	sb.WriteString(fmt.Sprintf("\n%s[N]ew [T]oggle [D]elete [P]riority [Q]uit [:]Cmd [?]Help[-:-:-]", dimC))

	// Update the TextView
	b.app.QueueUpdateDraw(func() {
		// Reset scroll position when updating content might be good
		b.todoPanel.ScrollToBeginning()
		b.todoPanel.SetText(sb.String())
	})
}

func (b *Baseline) updateFooter() {
	b.mu.RLock() // Read lock for notifications and focus state
	// Copy needed data under lock
	currentFocus := b.currentFocus
	var latest Notification
	hasNotifications := len(b.notifications) > 0
	if hasNotifications {
		latest = b.notifications[len(b.notifications)-1]
	}
	b.mu.RUnlock()


	var content string

	if currentFocus == "command" {
		// Command input is handled by showing/hiding the InputField
		// Ensure the regular footer TextView is empty or hidden
		content = ""
		b.app.QueueUpdateDraw(func() {
			b.layout.ResizeItem(b.footer, 0, 0)   // Hide notification footer
			b.layout.ResizeItem(b.cmdInput, 1, 0) // Show command input
			b.footer.SetText(content) // Clear text just in case
		})
		return // Don't overwrite with notification logic below
	}

	// If not in command mode, show notifications
	if hasNotifications {
		var color string
		switch latest.Type {
		case "error":
			color = "[red]"
		case "success":
			color = "[green]"
		default: // info
			color = colorTag(b.theme.Main)
		}
		content = fmt.Sprintf("%s[%s] %s%s[-:-:-]", colorTag(b.theme.Dim), latest.Time.Format("15:04:05"), color, latest.Message)
	} else {
		content = fmt.Sprintf("%sPress ':' to enter command mode, '?' for help[-:-:-]", colorTag(b.theme.Dim))
	}

	// Update the TextView and ensure correct visibility
	b.app.QueueUpdateDraw(func() {
		b.layout.ResizeItem(b.footer, 1, 0)   // Show notification footer
		b.layout.ResizeItem(b.cmdInput, 0, 0) // Hide command input
		b.footer.SetText(content)
	})
}

// --- Actions & Event Handling ---

func (b *Baseline) addNotification(message, msgType string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.notifications = append(b.notifications, Notification{
		Message: message,
		Type:    msgType,
		Time:    time.Now(),
	})
	// Keep only the last 5 notifications
	if len(b.notifications) > 5 {
		b.notifications = b.notifications[len(b.notifications)-5:]
	}
	// Trigger footer update after adding notification
	// Need to do this async as we hold the lock here
	go b.updateFooter()
}

func (b *Baseline) processCommand(command string) {
	b.mu.Lock() // Lock for modifying state based on command
	defer b.mu.Unlock()

	command = strings.TrimSpace(strings.ToLower(command))
	if command == "" {
		return
	}

	b.commandHistory = append(b.commandHistory, command)
	if len(b.commandHistory) > 20 {
		b.commandHistory = b.commandHistory[len(b.commandHistory)-20:]
	}

	parts := strings.Fields(command)
	cmd := parts[0]
	args := parts[1:]

	needsTodoUpdate := false
	needsWeatherUpdate := false
	needsThemeUpdate := false

	switch cmd {
	case "help", "?":
		b.addNotification("Cmds: help, todo, weather, clear, exit, theme, shortcut", "info")
	case "exit", "quit", "q":
		// Stop is thread-safe
		b.app.Stop() // Gracefully stop the application
	case "clear":
		b.notifications = []Notification{}
		b.addNotification("Notifications cleared", "success")
	case "shortcut":
		b.addNotification("Shortcuts: N(ew), T(oggle), D(elete), P(rio), Q(uit), :(Cmd), ?(Help)", "info")
	case "theme":
		if len(args) == 1 {
			themeName := args[0]
			if newTheme, ok := themes[themeName]; ok {
				b.theme = newTheme
				needsThemeUpdate = true // Flag theme update
				b.addNotification(fmt.Sprintf("Theme changed to %s", themeName), "success")
			} else {
				available := []string{}
				for k := range themes {
					available = append(available, k)
				}
				b.addNotification(fmt.Sprintf("Unknown theme: %s. Available: %s", themeName, strings.Join(available, ", ")), "error")
			}
		} else {
			b.addNotification("Usage: theme <themename>", "error")
		}
	case "todo":
		if len(args) > 0 {
			subCmd := args[0]
			todoArgs := args[1:]
			switch subCmd {
			case "add":
				if len(todoArgs) > 0 {
					text := strings.Join(todoArgs, " ")
					b.todoItems = append(b.todoItems, TodoItem{Text: text, Done: false, Priority: "medium"})
					b.saveTodos()
					b.addNotification(fmt.Sprintf("Added todo: %s", text), "success")
					needsTodoUpdate = true
				} else {
					b.addNotification("Usage: todo add <task text>", "error")
				}
			case "toggle", "done":
				if len(todoArgs) == 1 {
					index, err := strconv.Atoi(todoArgs[0])
					if err == nil && index >= 1 && index <= len(b.todoItems) {
						b.todoItems[index-1].Done = !b.todoItems[index-1].Done
						b.saveTodos()
						b.addNotification(fmt.Sprintf("Toggled todo #%d", index), "success")
						needsTodoUpdate = true
					} else {
						b.addNotification(fmt.Sprintf("Invalid todo index: %s", todoArgs[0]), "error")
					}
				} else {
					b.addNotification("Usage: todo toggle <index>", "error")
				}
			case "delete", "rm":
				if len(todoArgs) == 1 {
					index, err := strconv.Atoi(todoArgs[0])
					if err == nil && index >= 1 && index <= len(b.todoItems) {
						deleted := b.todoItems[index-1]
						b.todoItems = append(b.todoItems[:index-1], b.todoItems[index:]...) // Slice trick to delete
						b.saveTodos()
						b.addNotification(fmt.Sprintf("Deleted todo: %s", deleted.Text), "success")
						needsTodoUpdate = true
					} else {
						b.addNotification(fmt.Sprintf("Invalid todo index: %s", todoArgs[0]), "error")
					}
				} else {
					b.addNotification("Usage: todo delete <index>", "error")
				}
			// Add "prio" subcommand later if needed
			default:
				b.addNotification(fmt.Sprintf("Unknown todo command: %s", subCmd), "error")
			}
		} else {
			b.addNotification("Todo commands: add, toggle, delete", "info")
		}
	case "weather":
		if len(args) > 0 && args[0] == "set" && len(args) > 1 {
			location := strings.Join(args[1:], " ")
			b.weatherLocation = location
			// TODO: Persist location? Maybe save to a config file?
			b.addNotification(fmt.Sprintf("Weather location set to: %s. Fetching...", location), "success")
			needsWeatherUpdate = true // Flag weather fetch
		} else {
			b.addNotification("Usage: weather set <location>", "error")
		}
	default:
		b.addNotification(fmt.Sprintf("Unknown command: %s", command), "error")
	}

	// Clear input field after processing (do this outside lock if possible, but needs app access)
	// It's generally safe to call tview methods from the main event loop or via QueueUpdateDraw
	b.cmdInput.SetText("")

	// Trigger updates outside the main lock if needed
	if needsTodoUpdate {
		go b.updateTodos() // Update UI async
	}
	if needsThemeUpdate {
		go b.applyTheme() // Apply theme async
	}
	if needsWeatherUpdate {
		go b.fetchWeather() // Fetch new weather in background async
	}
	// Footer update is triggered by addNotification
}

// Global input handler attached to the application
func (b *Baseline) inputHandler(event *tcell.EventKey) *tcell.EventKey {
	// Check focus first without lock, might avoid locking unnecessarily
	if b.app.GetFocus() == b.cmdInput {
		// Let InputField handle Enter/Escape etc.
		// We could add history navigation (Up/Down) here if needed
		return event
	}

	// Lock only if handling global keys that modify state
	b.mu.Lock()
	defer b.mu.Unlock()

	needsTodoUpdate := false
	needsFooterUpdate := true // Most actions add a notification

	// Global keybindings when dashboard has focus
	switch event.Rune() {
	case ':':
		b.currentFocus = "command"
		// updateFooter will handle showing/hiding input field
		b.app.SetFocus(b.cmdInput) // Set focus after changing state
		// needsFooterUpdate = true // Already true
		return nil // Consume the event
	case 'q':
		b.app.Stop() // Stop is thread-safe
		needsFooterUpdate = false // App is stopping
		return nil
	case '?':
		b.addNotification("Keys: N(ew), T(oggle), D(elete), P(rio), Q(uit), :(Cmd), ?(Help)", "info")
		// needsFooterUpdate = true // Already true
		return nil
	case 'n':
		b.addNotification("Use ':todo add <task>' to add a new task", "info")
		// needsFooterUpdate = true // Already true
		return nil
	case 't': // Toggle first uncompleted todo
		toggled := false
		for i := range b.todoItems {
			if !b.todoItems[i].Done {
				b.todoItems[i].Done = true
				b.saveTodos()
				b.addNotification(fmt.Sprintf("Completed: %s", b.todoItems[i].Text), "success")
				needsTodoUpdate = true
				toggled = true
				break
			}
		}
		if !toggled {
			b.addNotification("No pending tasks to toggle.", "info")
		}
		// needsFooterUpdate = true // Already true
		return nil
	case 'd': // Delete first completed todo
		deleted := false
		for i := range b.todoItems {
			if b.todoItems[i].Done {
				deletedText := b.todoItems[i].Text
				b.todoItems = append(b.todoItems[:i], b.todoItems[i+1:]...)
				b.saveTodos()
				b.addNotification(fmt.Sprintf("Deleted: %s", deletedText), "success")
				needsTodoUpdate = true
				deleted = true
				break
			}
		}
		if !deleted {
			b.addNotification("No completed tasks to delete.", "info")
		}
		// needsFooterUpdate = true // Already true
		return nil
	case 'p': // Cycle priority of first uncompleted todo
		cycled := false
		priorities := []string{"low", "medium", "high"}
		for i := range b.todoItems {
			if !b.todoItems[i].Done {
				currentPrio := strings.ToLower(b.todoItems[i].Priority)
				if currentPrio == "" { currentPrio = "medium" } // Default
				currentIdx := -1
				for idx, p := range priorities {
					if p == currentPrio {
						currentIdx = idx
						break
					}
				}
				if currentIdx != -1 {
					nextIdx := (currentIdx + 1) % len(priorities)
					b.todoItems[i].Priority = priorities[nextIdx]
					b.saveTodos()
					b.addNotification(fmt.Sprintf("Priority set to %s for: %s", priorities[nextIdx], b.todoItems[i].Text), "success")
					needsTodoUpdate = true
					cycled = true
				}
				break // Only cycle the first one
			}
		}
		if !cycled {
			b.addNotification("No pending tasks to change priority.", "info")
		}
		// needsFooterUpdate = true // Already true
		return nil
	default:
		// If not a recognized global key, don't need lock/updates
		needsFooterUpdate = false
	}

	// Trigger updates outside the lock if needed
	// Use goroutines to avoid blocking the input handler
	if needsTodoUpdate {
		go b.updateTodos()
	}
	if needsFooterUpdate {
		go b.updateFooter()
	}


	// Handle other keys like F5 for refresh if needed (outside the rune switch)
	// switch event.Key() {
	// case tcell.KeyF5:
	// 	go b.updateSystemInfo()
	// 	go b.fetchWeather()
	//  // Time updates frequently anyway
	//  // Todos only change on action
	// 	b.addNotification("Data refreshed", "info")
	//  go b.updateFooter()
	// 	return nil
	// }

	return event // Return event for default processing if not handled
}

// --- Main Loop ---

func (b *Baseline) Run() error {
	b.setupLayout()

	// Initial data fetch and UI update
	b.updateHeader()
	go b.updateSystemInfo() // Run initial fetch in background
	go b.fetchWeather()
	b.updateTime() // Initial time update
	b.updateTodos() // Initial todo list render
	b.updateFooter() // Initial footer state
	b.addNotification("Welcome to Baseline (Go version)", "info")

	// Periodic updates using tickers
	sysTicker := time.NewTicker(refreshInterval)
	defer sysTicker.Stop()
	weatherTicker := time.NewTicker(15 * time.Minute) // Weather less frequent
	defer weatherTicker.Stop()
	timeTicker := time.NewTicker(1 * time.Second) // Update time every second
	defer timeTicker.Stop()

	// Goroutine for handling periodic updates
	go func() {
		// Initial weather fetch delay (don't fetch immediately again)
		time.Sleep(2 * time.Second)

		for {
			select {
			case <-sysTicker.C:
				go b.updateSystemInfo() // Fetch in background
			case <-weatherTicker.C:
				go b.fetchWeather() // Fetch in background
			case <-timeTicker.C:
				// Time update is cheap, can do directly or queue if needed
				b.updateTime()
				// Also update header which contains date (cheap)
				// b.updateHeader() // Header doesn't change per second
				// Footer update is handled by actions or periodically if needed
				// b.updateFooter()
			// --- Fix Start ---
			// Removed case <-b.app.Context().Done():
			// The goroutine will exit when the main program exits.
			// --- Fix End ---

			// Add a way to stop this goroutine if the app stops?
			// A quit channel could be used, signaled before app.Stop()
			// Or rely on main program exit.
			}
		}
	}()

	// Set global input capture
	b.app.SetInputCapture(b.inputHandler)

	// Run the application
	// Set Root and Focus outside the Run() call
	b.app.SetRoot(b.layout, true).SetFocus(b.layout)
	if err := b.app.Run(); err != nil {
		// Log error before returning
		log.Printf("Error running application: %v", err)
		return fmt.Errorf("failed to run application: %w", err)
	}

	// Application stopped gracefully
	log.Println("Application stopped.")
	return nil
}

// --- Entry Point ---

func main() {
	// Clear the screen first for better visibility
	clearScreen()

	// Print startup message to terminal
	fmt.Println("Starting Baseline application...")

	// Optional: Set terminal title (might not work everywhere)
	fmt.Print("\033]0;Baseline (Go)\007")

	// Basic logging setup
	logFilename := "baseline_debug.log"
	logFile, err := os.OpenFile(logFilename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		log.SetOutput(logFile)
		log.SetFlags(log.LstdFlags | log.Lshortfile)
		defer logFile.Close()
		log.Println("--- Application Starting ---")
	} else {
		// Fallback to stderr if log file fails
		log.SetOutput(os.Stderr)
		log.Printf("Warning: Could not open log file '%s': %v. Logging to stderr.", logFilename, err)
	}

	// Print initialization message
	fmt.Println("Initializing TUI components...")

	baselineApp := NewBaseline()
	fmt.Println("Starting TUI application. Press 'q' to quit.")
	
	if err := baselineApp.Run(); err != nil {
		// Error should already be logged by Run() before returning
		fmt.Fprintf(os.Stderr, "Application exited with error: %v\n", err)
		os.Exit(1)
	}
	log.Println("--- Application Exited Gracefully ---")
	fmt.Println("Baseline exited.") // Message to user terminal
}

// --- Helper for platform specific clear ---
// (Not strictly needed with tview's screen management, but can be useful)
func clearScreen() {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "cls")
	} else {
		cmd = exec.Command("clear")
	}
	cmd.Stdout = os.Stdout
	_ = cmd.Run() // Ignore error
}