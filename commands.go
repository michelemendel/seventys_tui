package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type commandResultMsg struct {
	response   string
	clear      bool
	fastPrint  bool
}

func parseCommandLine(line string) (cmd, args string) {
	line = strings.TrimSpace(strings.ToUpper(line))
	if line == "" {
		return "", ""
	}
	fields := strings.Fields(line)
	cmd = fields[0]
	if len(fields) > 1 {
		args = strings.TrimSpace(line[len(cmd):])
	}
	return cmd, args
}

func runCommand(line string) tea.Cmd {
	return func() tea.Msg {
		cmd, args := parseCommandLine(line)
		switch cmd {
		case "":
			return commandResultMsg{}
		case "HELP", "?":
			return commandResultMsg{response: helpText(), fastPrint: true}
		case "TIME", "DATE":
			return commandResultMsg{response: cmdTime()}
		case "LIST", "DIR":
			return commandResultMsg{response: cmdList(args), fastPrint: true}
		case "WEATHER":
			return commandResultMsg{response: cmdWeather(args)}
		case "WHOAMI":
			return commandResultMsg{response: cmdWhoami()}
		case "PWD":
			return commandResultMsg{response: cmdPwd()}
		case "STATUS":
			return commandResultMsg{response: cmdStatus()}
		case "CAL":
			return commandResultMsg{response: cmdCal()}
		case "UPTIME":
			return commandResultMsg{response: cmdUptime()}
		case "ADVICE":
			return commandResultMsg{response: cmdAdvice()}
		case "PHASE":
			return commandResultMsg{response: cmdMoonPhase()}
		case "ISS":
			return commandResultMsg{response: cmdISS()}
		case "QUOTE":
			return commandResultMsg{response: cmdQuote()}
		case "HELLO", "HI":
			return commandResultMsg{response: "GREETINGS, OPERATOR. THE MAINFRAME AWAITS YOUR INPUT."}
		case "CLEAR", "CLS":
			return commandResultMsg{clear: true}
		default:
			return commandResultMsg{response: "I DON'T UNDERSTAND"}
		}
	}
}

func helpText() string {
	return strings.TrimRight(`
AVAILABLE COMMANDS:

  HELP     - SHOW THIS LIST
  EXIT     - DISCONNECT FROM MAINFRAME
  TIME     - LOCAL SYSTEM TIME
  LIST     - DIRECTORY LISTING (LS -LA)
  WEATHER  - TODAY'S WEATHER [CITY]
  WHOAMI   - CURRENT OPERATOR ID
  PWD      - WORKING DIRECTORY
  STATUS   - SYSTEM DIAGNOSTICS
  CAL      - MONTHLY CALENDAR
  UPTIME   - SYSTEM UPTIME
  ADVICE   - REQUEST GUIDANCE FROM CENTRAL
  PHASE    - LUNAR PHASE REPORT
  ISS      - ORBITAL SCAN (SPACE STATION)
  QUOTE    - RANDOM TRANSMISSION
  CLEAR    - CLEAR TERMINAL SCREEN
  HELLO    - OPEN CHANNEL GREETING
`, "\n")
}

func cmdTime() string {
	return strings.ToUpper(time.Now().Format("MONDAY 02 JANUARY 2006 15:04:05 MST"))
}

func cmdList(args string) string {
	cmdArgs := []string{"-la"}
	if args != "" {
		cmdArgs = append(cmdArgs, args)
	}
	out, err := runShell("ls", cmdArgs...)
	if err != nil {
		return fmt.Sprintf("ERROR: %s", strings.ToUpper(err.Error()))
	}
	return out
}

func cmdWhoami() string {
	u, err := user.Current()
	if err != nil {
		return fmt.Sprintf("ERROR: %s", strings.ToUpper(err.Error()))
	}
	return strings.ToUpper(u.Username)
}

func cmdPwd() string {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Sprintf("ERROR: %s", strings.ToUpper(err.Error()))
	}
	return strings.ToUpper(dir)
}

func cmdStatus() string {
	hostname, _ := os.Hostname()
	return fmt.Sprintf("ALL SYSTEMS NOMINAL\nHOST: %s\nAUDIO COUPLER: ONLINE\nTELETYPE: ENGAGED",
		strings.ToUpper(hostname))
}

func cmdCal() string {
	out, err := runShell("cal")
	if err != nil {
		return fmt.Sprintf("ERROR: %s", strings.ToUpper(err.Error()))
	}
	return out
}

func cmdUptime() string {
	out, err := runShell("uptime")
	if err != nil {
		return fmt.Sprintf("ERROR: %s", strings.ToUpper(err.Error()))
	}
	return out
}

func cmdAdvice() string {
	body, err := httpGet("https://api.adviceslip.com/advice", 5*time.Second)
	if err != nil {
		return fmt.Sprintf("ERROR: UNABLE TO REACH ADVICE NETWORK\n%s", strings.ToUpper(err.Error()))
	}

	var result struct {
		Slip struct {
			Advice string `json:"advice"`
		} `json:"slip"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "ERROR: ADVICE TRANSMISSION CORRUPTED"
	}
	return "CENTRAL ADVICE: " + strings.ToUpper(result.Slip.Advice)
}

func cmdMoonPhase() string {
	phase, illumination := localMoonPhase(time.Now().UTC())
	return fmt.Sprintf("LUNAR PHASE: %s\nILLUMINATION INDEX: %.2F\nSOURCE: LOCAL LUNAR SENSOR",
		phase, illumination)
}

func localMoonPhase(t time.Time) (phase string, illumination float64) {
	const synodicMonth = 29.53058867
	knownNewMoon := time.Date(2000, 1, 6, 18, 14, 0, 0, time.UTC)

	age := math.Mod(t.Sub(knownNewMoon).Hours()/24, synodicMonth)
	if age < 0 {
		age += synodicMonth
	}

	illumination = (1 - math.Cos(2*math.Pi*age/synodicMonth)) / 2

	switch {
	case age < 1.84566:
		phase = "NEW MOON"
	case age < 5.53699:
		phase = "WAXING CRESCENT"
	case age < 9.22831:
		phase = "FIRST QUARTER"
	case age < 12.91963:
		phase = "WAXING GIBBOUS"
	case age < 16.61096:
		phase = "FULL MOON"
	case age < 20.30228:
		phase = "WANING GIBBOUS"
	case age < 23.99361:
		phase = "LAST QUARTER"
	default:
		phase = "WANING CRESCENT"
	}

	return phase, illumination
}

func cmdISS() string {
	body, err := httpGet("http://api.open-notify.org/iss-now.json", 5*time.Second)
	if err != nil {
		return fmt.Sprintf("ERROR: ORBITAL TRACKING OFFLINE\n%s", strings.ToUpper(err.Error()))
	}

	var result struct {
		IssPosition struct {
			Latitude  string `json:"latitude"`
			Longitude string `json:"longitude"`
		} `json:"iss_position"`
		Timestamp int64 `json:"timestamp"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "ERROR: ORBITAL DATA CORRUPTED"
	}

	return fmt.Sprintf("ORBITAL SCAN COMPLETE\nSATELLITE: INTERNATIONAL SPACE STATION\nLATITUDE:  %s\nLONGITUDE: %s",
		result.IssPosition.Latitude, result.IssPosition.Longitude)
}

func cmdQuote() string {
	body, err := httpGet("https://api.quotable.io/quotes/random", 5*time.Second)
	if err == nil {
		var quotes []struct {
			Content string `json:"content"`
			Author  string `json:"author"`
		}
		if json.Unmarshal(body, &quotes) == nil && len(quotes) > 0 {
			return fmt.Sprintf("INCOMING TRANSMISSION:\n\"%s\"\n-- %s",
				strings.ToUpper(quotes[0].Content), strings.ToUpper(quotes[0].Author))
		}
	}

	fallback := []string{
		"OPEN THE POD BAY DOORS, HAL.",
		"I'M SORRY DAVE, I'M AFRAID I CAN'T DO THAT.",
		"THE FUTURE IS ALREADY HERE — IT'S JUST NOT EVENLY DISTRIBUTED.",
		"ALL THESE WORLDS ARE YOURS EXCEPT EUROPA.",
		"THE MATRIX HAS YOU.",
	}
	return "INCOMING TRANSMISSION:\n\"" + fallback[rand.Intn(len(fallback))] + "\""
}

func cmdWeather(city string) string {
	if city == "" {
		city = "LONDON"
	}

	geoURL := fmt.Sprintf(
		"https://geocoding-api.open-meteo.com/v1/search?name=%s&count=1&language=en&format=json",
		url.QueryEscape(city),
	)
	body, err := httpGet(geoURL, 5*time.Second)
	if err != nil {
		return fmt.Sprintf("ERROR: WEATHER SATELLITE OFFLINE\n%s", strings.ToUpper(err.Error()))
	}

	var geo struct {
		Results []struct {
			Name      string  `json:"name"`
			Country   string  `json:"country"`
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &geo); err != nil || len(geo.Results) == 0 {
		return fmt.Sprintf("ERROR: LOCATION '%s' NOT FOUND", city)
	}

	loc := geo.Results[0]
	weatherURL := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast?latitude=%f&longitude=%f&current=temperature_2m,weather_code,wind_speed_10m&timezone=auto",
		loc.Latitude, loc.Longitude,
	)
	body, err = httpGet(weatherURL, 5*time.Second)
	if err != nil {
		return fmt.Sprintf("ERROR: WEATHER FEED INTERRUPTED\n%s", strings.ToUpper(err.Error()))
	}

	var weather struct {
		Current struct {
			Temperature float64 `json:"temperature_2m"`
			WeatherCode int     `json:"weather_code"`
			WindSpeed   float64 `json:"wind_speed_10m"`
		} `json:"current"`
	}
	if err := json.Unmarshal(body, &weather); err != nil {
		return "ERROR: WEATHER DATA CORRUPTED"
	}

	return fmt.Sprintf(
		"WEATHER REPORT: %s, %s\nTEMPERATURE: %.0F C\nCONDITIONS: %s\nWIND: %.0F KM/H",
		strings.ToUpper(loc.Name),
		strings.ToUpper(loc.Country),
		weather.Current.Temperature,
		strings.ToUpper(weatherDescription(weather.Current.WeatherCode)),
		weather.Current.WindSpeed,
	)
}

func weatherDescription(code int) string {
	switch code {
	case 0:
		return "clear sky"
	case 1, 2, 3:
		return "partly cloudy"
	case 45, 48:
		return "fog"
	case 51, 53, 55:
		return "drizzle"
	case 61, 63, 65:
		return "rain"
	case 71, 73, 75:
		return "snow"
	case 80, 81, 82:
		return "rain showers"
	case 95, 96, 99:
		return "thunderstorm"
	default:
		return "unknown"
	}
}

func runShell(name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil {
		return "", err
	}
	return strings.ToUpper(strings.TrimRight(string(out), "\n")), nil
}

func httpGet(endpoint string, timeout time.Duration) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}
