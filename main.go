package main

import (
	"embed"
	"encoding/base64"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"math"
	"net/http"
	"net/http/fcgi"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

//go:embed templates/marquee.svg
var templatesFS embed.FS

var (
	tmpl         *template.Template
	cacheMu      sync.RWMutex
	cssCache     = make(map[string]CacheEntry)
	client       = &http.Client{Timeout: 10 * time.Second}
	fontURLRegex = regexp.MustCompile(`url\(['"]?(https://fonts\.gstatic\.com/[^'")]+)['"]?\)\s*format\(['"]([^'"]+)['"]\)`)
)

type CacheEntry struct {
	CSS       string
	CreatedAt time.Time
}

type SVGParams struct {
	Width        int
	Height       int
	Font         string
	Weight       string
	FontSize     int
	Color        string
	Duration     string
	BaseText     string
	ExactWidth   int
	ExactWidthX2 int
	WidthX4      int
	Bg           string
	Rotate       float64
	CX           float64
	CY           float64
	RectY        float64
	RectHeight   float64
	TextY        float64
	FontCSS      template.CSS
}

func init() {
	var err error
	tmpl, err = template.ParseFS(templatesFS, "templates/marquee.svg")
	if err != nil {
		log.Fatalf("failed to parse template: %v", err)
	}
}

func formatColor(c string) string {
	if c == "" || c == "transparent" {
		return ""
	}
	if strings.HasPrefix(c, "#") {
		return c
	}
	
	// Check if the string is hexadecimal
	isHex := true
	for _, r := range c {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			isHex = false
			break
		}
	}
	
	// If hex and of valid length, add the '#' prefix
	if isHex && (len(c) == 3 || len(c) == 4 || len(c) == 6 || len(c) == 8) {
		return "#" + c
	}
	
	return c
}

// parseDuration parses and formats the animation duration string.
// If it's a numeric string, it decides whether to treat it as seconds or milliseconds.
func parseDuration(durationStr string) string {
	if durationStr == "" {
		return "20s"
	}
	if strings.HasSuffix(durationStr, "s") || strings.HasSuffix(durationStr, "ms") {
		return durationStr
	}
	
	if val, err := strconv.Atoi(durationStr); err == nil {
		if val > 100 {
			return fmt.Sprintf("%dms", val)
		}
		return fmt.Sprintf("%ds", val)
	}
	
	return durationStr
}

// fetchFontCSS gets Google Font CSS (subsetted to the given text) and replaces font file URLs with base64 Data URIs.
func fetchFontCSS(font, weight, text string) string {
	cacheKey := fmt.Sprintf("%s:%s:%s", font, weight, text)
	
	cacheMu.RLock()
	entry, found := cssCache[cacheKey]
	cacheMu.RUnlock()
	
	if found && time.Since(entry.CreatedAt) < 24*time.Hour {
		return entry.CSS
	}
	
	// Build request URL for Google Fonts subset CSS
	params := url.Values{}
	params.Set("family", fmt.Sprintf("%s:wght@%s", font, weight))
	params.Set("text", text)
	params.Set("display", "fallback")
	
	cssURL := "https://fonts.googleapis.com/css2?" + params.Encode()
	
	req, err := http.NewRequest("GET", cssURL, nil)
	if err != nil {
		return ""
	}
	
	// Use modern User-Agent to receive WOFF2 files (best compression for SVG embedding)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error fetching Google Font CSS: %v", err)
		return ""
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		log.Printf("Google Fonts CSS API returned status %d", resp.StatusCode)
		return ""
	}
	
	cssBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}
	
	css := string(cssBytes)
	encodedCSS := encodeFonts(css)
	
	// Cache the result
	cacheMu.Lock()
	if len(cssCache) > 1000 {
		// Evict/clear cache if too large
		cssCache = make(map[string]CacheEntry)
	}
	cssCache[cacheKey] = CacheEntry{
		CSS:       encodedCSS,
		CreatedAt: time.Now(),
	}
	cacheMu.Unlock()
	
	return encodedCSS
}

// encodeFonts replaces external .gstatic.com font file URLs in the CSS with base64 Data URIs.
func encodeFonts(css string) string {
	matches := fontURLRegex.FindAllStringSubmatch(css, -1)
	if len(matches) == 0 {
		return css
	}
	
	downloaded := make(map[string]string)
	
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		fontURL := match[1]
		fontFormat := match[2]
		
		dataURI, exists := downloaded[fontURL]
		if !exists {
			fontData, err := downloadFontFile(fontURL)
			if err != nil {
				log.Printf("Failed to download font file from %s: %v", fontURL, err)
				continue
			}
			encoded := base64.StdEncoding.EncodeToString(fontData)
			dataURI = fmt.Sprintf("data:font/%s;base64,%s", fontFormat, encoded)
			downloaded[fontURL] = dataURI
		}
		
		css = strings.ReplaceAll(css, fontURL, dataURI)
	}
	
	return css
}

// downloadFontFile downloads a binary font file from gstatic.
func downloadFontFile(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP status %d", resp.StatusCode)
	}
	
	return io.ReadAll(resp.Body)
}

func handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	
	query := r.URL.Query()
	
	font := query.Get("font")
	if font == "" {
		font = "Syne"
	}
	
	weight := query.Get("weight")
	if weight == "" {
		weight = "600"
	}
	
	fontSizeStr := query.Get("size")
	fontSize := 48
	if fontSizeStr != "" {
		if val, err := strconv.Atoi(fontSizeStr); err == nil && val > 0 {
			fontSize = val
		}
	}
	
	duration := parseDuration(query.Get("duration"))
	
	widthStr := query.Get("width")
	width := 1000
	if widthStr != "" {
		if val, err := strconv.Atoi(widthStr); err == nil && val > 0 {
			width = val
		}
	}
	
	heightStr := query.Get("height")
	height := 200
	if heightStr != "" {
		if val, err := strconv.Atoi(heightStr); err == nil && val > 0 {
			height = val
		}
	}
	
	content := query.Get("content")
	if content == "" {
		content = "YOUR • TEXT • GOES • HERE • "
	}
	
	color := query.Get("color")
	if color == "" {
		color = query.Get("pcolor")
	}
	if color == "" {
		color = "#FFFFFF"
	}
	color = formatColor(color)
	
	bg := query.Get("bg")
	if bg == "" {
		bg = query.Get("background")
	}
	if bg == "" {
		bg = "#000000"
	}
	bg = formatColor(bg)
	
	rotateStr := query.Get("rotate")
	rotate := -3.0
	if rotateStr != "" {
		if val, err := strconv.ParseFloat(rotateStr, 64); err == nil {
			rotate = val
		}
	}
	
	widthFactorStr := query.Get("width_factor")
	widthFactor := 0.52
	if widthFactorStr != "" {
		if val, err := strconv.ParseFloat(widthFactorStr, 64); err == nil && val > 0 {
			widthFactor = val
		}
	}
	
	fontCSS := fetchFontCSS(font, weight, content)
	
	// Compute positioning
	cx := float64(width) / 2.0
	cy := float64(height) / 2.0
	
	rectHeight := float64(fontSize) * 1.875
	rectY := cy - rectHeight/2.0
	textY := cy + float64(fontSize)*0.375
	
	// Calculate exact translation width for seamless SMIL looping
	exactWidth := int(math.Round(float64(utf8.RuneCountInString(content)) * float64(fontSize) * widthFactor))
	
	params := SVGParams{
		Width:        width,
		Height:       height,
		Font:         font,
		Weight:       weight,
		FontSize:     fontSize,
		Color:        color,
		Duration:     duration,
		BaseText:     content,
		ExactWidth:   exactWidth,
		ExactWidthX2: exactWidth * 2,
		WidthX4:      width * 4,
		Bg:           bg,
		Rotate:       rotate,
		CX:           cx,
		CY:           cy,
		RectY:        rectY,
		RectHeight:   rectHeight,
		TextY:        textY,
		FontCSS:      template.CSS(fontCSS),
	}
	
	w.Header().Set("Content-Type", "image/svg+xml")
	// Caching for 1 year
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	
	err := tmpl.Execute(w, params)
	if err != nil {
		log.Printf("Template execution error: %v", err)
	}
}

func main() {
	localFlag := flag.Bool("local", false, "Serve with http.ListenAndServe on port 8080 rather than FCGI")
	flag.BoolVar(localFlag, "dev", *localFlag, "Serve with http.ListenAndServe on port 8080 rather than FCGI (alias for -local)")
	flag.Parse()
	
	http.HandleFunc("/", handler)
	
	if *localFlag {
		log.Println("Starting local HTTP server on :8080...")
		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Fatalf("ListenAndServe error: %v", err)
		}
	} else {
		if err := fcgi.Serve(nil, nil); err != nil {
			log.Fatalf("FCGI Serve error: %v", err)
		}
	}
}
