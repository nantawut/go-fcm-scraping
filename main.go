package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// --- Configuration ---

var (
	// Team data remains as a package-level variable as it's static configuration.
	teams = []Team{
		{"Bradford City", "https://www.fifacm.com/25/team/1804/bradford-city"},
		{"Doncaster Rovers", "https://www.fifacm.com/25/team/142/doncaster-rovers"},
		{"Carlisle United", "https://www.fifacm.com/25/team/1480/carlisle-united"},
		{"Swindon Town", "https://www.fifacm.com/25/team/1934/swindon-town"},
		{"Chesterfield", "https://www.fifacm.com/25/team/1924/chesterfield"},
		{"Tranmere Rovers", "https://www.fifacm.com/25/team/15048/tranmere-rovers"},
		{"Crewe Alexandra", "https://www.fifacm.com/25/team/121/crewe-alexandra"},
		{"Walsall", "https://www.fifacm.com/25/team/1803/walsall"},
		{"Notts County", "https://www.fifacm.com/25/team/1937/notts-county"},
		{"Port Vale", "https://www.fifacm.com/25/team/1928/port-vale"},
		{"Grimsby Town", "https://www.fifacm.com/25/team/92/grimsby-town"},
		{"Gillingham", "https://www.fifacm.com/25/team/1802/gillingham"},
		{"Cheltenham Town", "https://www.fifacm.com/25/team/1936/cheltenham-town"},
		{"Milton Keynes Dons", "https://www.fifacm.com/25/team/1798/milton-keynes-dons"},
		{"AFC Wimbledon", "https://www.fifacm.com/25/team/112259/afc-wimbledon"},
		{"Salford City", "https://www.fifacm.com/25/team/113926/salford-city"},
		{"Newport County", "https://www.fifacm.com/25/team/112254/newport-county"},
		{"Bromley", "https://www.fifacm.com/25/team/112764/bromley"},
		{"Barrow", "https://www.fifacm.com/25/team/381/barrow"},
		{"Harrogate Town", "https://www.fifacm.com/25/team/112222/harrogate-town"},
		{"Fleetwood Town", "https://www.fifacm.com/25/team/112260/fleetwood-town"},
		{"Morecambe", "https://www.fifacm.com/25/team/357/morecambe"},
		{"Accrington Stanley", "https://www.fifacm.com/25/team/110313/accrington-stanley"},
		{"Colchester United", "https://www.fifacm.com/25/team/1935/colchester-united"},
	}

	userAgents = []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:89.0) Gecko/20100101 Firefox/89.0",
	}
)

// --- Data Structures ---

// Team holds the static information for a team.
type Team struct {
	Name string
	URL  string
}

// Player holds the scraped data for a player.
// REFACTOR: Fields are now typed correctly (e.g., int for Age, Potential).
// This provides type safety and makes the data easier to work with.
type Player struct {
	Profile   string `json:"profile"`
	Team      string `json:"team"`
	Price     string `json:"price"`
	Age       int    `json:"age"`
	Overall   int    `json:"overall"`
	Potential int    `json:"potential"`
	Growth    int    `json:"growth"`
}

// --- Scraper ---

// Scraper encapsulates the state and methods for the scraping job.
// REFACTOR: This avoids global variables, making dependencies explicit and the code testable.
type Scraper struct {
	client       *http.Client
	minPotential int
	minGrowth    int
	outputFile   string
	concurrency  int
	minDelay     time.Duration
	maxDelay     time.Duration
	rand         *rand.Rand // Use a local rand instance to avoid global state.

	// REFACTOR: Regexes are compiled once for performance.
	rowPattern  *regexp.Regexp
	cellPattern *regexp.Regexp
	tagStripper *regexp.Regexp
}

// NewScraper creates and configures a new Scraper instance.
func NewScraper() *Scraper {
	// REFACTOR: rand.Seed is deprecated. Create a new source for our local rand instance.
	source := rand.NewSource(time.Now().UnixNano())

	return &Scraper{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		minPotential: 70,
		minGrowth:    12,
		outputFile:   "high_potential_players.json", // Outputting valid JSON now
		concurrency:  3,
		minDelay:     2 * time.Second,
		maxDelay:     5 * time.Second,
		rand:         rand.New(source),
		rowPattern:   regexp.MustCompile(`<tr.*?>.*?</tr>`),
		cellPattern:  regexp.MustCompile(`<td.*?>(.*?)</td>`),
		tagStripper:  regexp.MustCompile(`<.*?>`),
	}
}

// fetchHTML fetches the HTML content from a given URL.
func (s *Scraper) fetchHTML(url string) (string, error) {
	// Random delay to avoid triggering rate limits.
	delay := s.minDelay + time.Duration(s.rand.Int63n(int64(s.maxDelay-s.minDelay)))
	time.Sleep(delay)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", userAgents[s.rand.Intn(len(userAgents))])
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP request failed with status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response body failed: %w", err)
	}

	return string(body), nil
}

// extractPlayers parses the HTML to find players matching the criteria.
func (s *Scraper) extractPlayers(team Team, html string) []Player {
	var players []Player
	rows := s.rowPattern.FindAllString(html, -1)

	for _, row := range rows {
		cols := s.cellPattern.FindAllStringSubmatch(row, -1)
		if len(cols) < 6 {
			continue
		}

		profile := s.stripTags(cols[0][1])
		if strings.Contains(profile, "Loan") {
			continue
		}

		potential, err := strconv.Atoi(s.stripTags(cols[2][1]))
		if err != nil || potential < s.minPotential {
			continue
		}

		growth, err := strconv.Atoi(s.stripTags(cols[3][1]))
		if err != nil || growth < s.minGrowth {
			continue
		}

		// REFACTOR: Parse all numeric fields and handle errors gracefully.
		overall, _ := strconv.Atoi(s.stripTags(cols[1][1]))
		age, _ := strconv.Atoi(s.stripTags(cols[4][1]))
		price := s.stripTags(cols[5][1])

		players = append(players, Player{
			Profile:   profile,
			Team:      team.Name,
			Price:     price,
			Age:       age,
			Overall:   overall,
			Potential: potential,
			Growth:    growth,
		})
	}
	return players
}

// stripTags removes HTML tags from a string.
func (s *Scraper) stripTags(input string) string {
	return strings.TrimSpace(s.tagStripper.ReplaceAllString(input, ""))
}

// writePlayersToFile saves the list of players to a valid JSON file.
// REFACTOR: This now writes a proper JSON array, making the output much more useful.
func (s *Scraper) writePlayersToFile(players []Player) error {
	// Marshal the entire slice into a valid JSON array format with indentation.
	jsonData, err := json.MarshalIndent(players, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal players to JSON: %w", err)
	}

	// Use os.WriteFile for a simpler file writing operation.
	return os.WriteFile(s.outputFile, jsonData, 0644)
}

// processTeam is the worker function for a single team.
func (s *Scraper) processTeam(team Team, results chan<- Player) {
	html, err := s.fetchHTML(team.URL)
	if err != nil {
		log.Printf("Error fetching %s: %v\n", team.Name, err)
		return
	}

	players := s.extractPlayers(team, html)
	for _, p := range players {
		results <- p
	}
}

// Run starts the entire scraping process.
func (s *Scraper) Run(teams []Team) {
	startTime := time.Now()
	log.Println("Starting player scouting...")

	var wg sync.WaitGroup
	// REFACTOR: The results channel now transports single players for more granular processing.
	results := make(chan Player, len(teams)) // Buffer is still useful.
	allPlayers := make([]Player, 0)

	// REFACTOR: The result collection is now a simple for...range loop.
	// It will block until the channel is closed by the goroutine above.
	go func() {
		for player := range results {
			allPlayers = append(allPlayers, player)
		}
	}()

	semaphore := make(chan struct{}, s.concurrency)
	for _, team := range teams {
		wg.Add(1)
		semaphore <- struct{}{} // Acquire semaphore

		go func(t Team) {
			defer wg.Done()
			s.processTeam(t, results)
			<-semaphore // Release semaphore
		}(team)
	}

	// Wait for the collection to finish.
	// Since wg.Wait() happens before `close(results)`, and the collection loop
	// only finishes after the channel is closed, we can be sure all players are collected
	// once the main goroutine unblocks here. A better way is to use another waitgroup
	// for the collector.
	var collectorWg sync.WaitGroup
	collectorWg.Add(1)
	go func() {
		defer collectorWg.Done()
		for player := range results {
			allPlayers = append(allPlayers, player)
		}
	}()

	wg.Wait()
	close(results)
	collectorWg.Wait() // Wait for the collector to finish.

	if err := s.writePlayersToFile(allPlayers); err != nil {
		log.Printf("Error writing to file: %v\n", err)
	} else {
		log.Printf("Results saved to %s\n", s.outputFile)
	}

	log.Printf("\nScouting completed in %v\n", time.Since(startTime))
	log.Printf("Found %d players with potential >= %d\n", len(allPlayers), s.minPotential)
}

func main() {
	scraper := NewScraper()
	scraper.Run(teams)
}
