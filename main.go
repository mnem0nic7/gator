package main

import (
	"context"
	"database/sql"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"gator/internal/api"
	"gator/internal/config"
	"gator/internal/database"
	"gator/internal/tui"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

// state struct holds a pointer to a config and database
type state struct {
	db  *database.Queries
	cfg *config.Config
}

// command represents a parsed CLI command
type command struct {
	name string
	args []string
}

// commands struct holds all the commands the CLI can handle
type commands struct {
	handlers map[string]func(*state, command) error
}

// RSSFeed represents the structure of an RSS feed
type RSSFeed struct {
	Channel struct {
		Title       string    `xml:"title"`
		Link        string    `xml:"link"`
		Description string    `xml:"description"`
		Item        []RSSItem `xml:"item"`
	} `xml:"channel"`
}

// RSSItem represents a single item in an RSS feed
type RSSItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
}

// middlewareLoggedIn wraps handlers that require a logged-in user
// It provides the user as a parameter to avoid duplicating authentication code
func middlewareLoggedIn(handler func(s *state, cmd command, user database.User) error) func(*state, command) error {
	return func(s *state, cmd command) error {
		// Get current user from config
		currentUser := s.cfg.CurrentUser
		if currentUser == "" {
			return fmt.Errorf("no user is currently logged in")
		}

		// Get user from database
		user, err := s.db.GetUser(context.Background(), currentUser)
		if err != nil {
			return fmt.Errorf("couldn't get current user: %w", err)
		}

		// Call the wrapped handler with the user
		return handler(s, cmd, user)
	}
}

// run method runs a given command with the provided state if it exists
func (c *commands) run(s *state, cmd command) error {
	handler, exists := c.handlers[cmd.name]
	if !exists {
		return fmt.Errorf("unknown command: %s", cmd.name)
	}
	return handler(s, cmd)
}

// register method registers a new handler function for a command name
func (c *commands) register(name string, f func(*state, command) error) {
	c.handlers[name] = f
}

// fetchFeed fetches an RSS feed from the given URL and returns a parsed RSSFeed struct
func fetchFeed(ctx context.Context, feedURL string) (*RSSFeed, error) {
	// Create HTTP request with context
	req, err := http.NewRequestWithContext(ctx, "GET", feedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("couldn't create request: %w", err)
	}

	// Set User-Agent header to identify our program
	req.Header.Set("User-Agent", "gator")

	// Create HTTP client and make request
	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("couldn't make request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("couldn't read response body: %w", err)
	}

	// Parse XML into RSSFeed struct
	var feed RSSFeed
	err = xml.Unmarshal(body, &feed)
	if err != nil {
		return nil, fmt.Errorf("couldn't unmarshal XML: %w", err)
	}

	// Unescape HTML entities in channel fields
	feed.Channel.Title = html.UnescapeString(feed.Channel.Title)
	feed.Channel.Description = html.UnescapeString(feed.Channel.Description)

	// Unescape HTML entities in item fields
	for i := range feed.Channel.Item {
		feed.Channel.Item[i].Title = html.UnescapeString(feed.Channel.Item[i].Title)
		feed.Channel.Item[i].Description = html.UnescapeString(feed.Channel.Item[i].Description)
	}

	return &feed, nil
}

// handlerRegister handles the register command
func handlerRegister(s *state, cmd command) error {
	if len(cmd.args) == 0 {
		return fmt.Errorf("usage: %s <username>", cmd.name)
	}

	username := cmd.args[0]

	// Create new user
	now := time.Now().UTC()
	userParams := database.CreateUserParams{
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
		Name:      username,
	}

	user, err := s.db.CreateUser(context.Background(), userParams)
	if err != nil {
		return fmt.Errorf("couldn't create user: %w", err)
	}

	// Set current user in config
	err = s.cfg.SetUser(username)
	if err != nil {
		return fmt.Errorf("couldn't set current user: %w", err)
	}

	fmt.Printf("User %s has been created\n", username)
	fmt.Printf("User data: %+v\n", user)
	return nil
}

// handlerLogin handles the login command
func handlerLogin(s *state, cmd command) error {
	if len(cmd.args) == 0 {
		return fmt.Errorf("usage: %s <username>", cmd.name)
	}

	username := cmd.args[0]

	// Check if user exists in database
	_, err := s.db.GetUser(context.Background(), username)
	if err != nil {
		return fmt.Errorf("user %s doesn't exist", username)
	}

	// Set current user in config
	err = s.cfg.SetUser(username)
	if err != nil {
		return fmt.Errorf("couldn't set current user: %w", err)
	}

	fmt.Printf("User has been set to: %s\n", username)
	return nil
}

// handlerReset handles the reset command
func handlerReset(s *state, cmd command) error {
	err := s.db.ResetUsers(context.Background())
	if err != nil {
		return fmt.Errorf("couldn't reset users: %w", err)
	}

	fmt.Println("Database reset successfully!")
	return nil
}

// handlerUsers handles the users command
func handlerUsers(s *state, cmd command) error {
	users, err := s.db.GetUsers(context.Background())
	if err != nil {
		return fmt.Errorf("couldn't get users: %w", err)
	}

	currentUser := s.cfg.CurrentUser

	for _, user := range users {
		if user.Name == currentUser {
			fmt.Printf("* %s (current)\n", user.Name)
		} else {
			fmt.Printf("* %s\n", user.Name)
		}
	}

	return nil
}

// Enhanced handlerAgg to fetch feeds concurrently
func handlerAgg(s *state, cmd command) error {
	if len(cmd.args) < 1 {
		return fmt.Errorf("usage: agg <time_between_reqs>")
	}

	timeBetweenReqs, err := time.ParseDuration(cmd.args[0])
	if err != nil {
		return fmt.Errorf("invalid duration: %v", err)
	}

	fmt.Printf("Collecting feeds every %s\n", timeBetweenReqs)
	ticker := time.NewTicker(timeBetweenReqs)
	defer ticker.Stop()

	feedChan := make(chan struct{}, 5) // Limit concurrency to 5 feeds at a time

	for {
		feedChan <- struct{}{} // Block if limit is reached
		go func() {
			scrapeFeeds(s)
			<-feedChan // Release slot after completion
		}()
		<-ticker.C
	}
}

// handlerAddfeed handles the addfeed command to create new feeds
func handlerAddfeed(s *state, cmd command, user database.User) error {
	if len(cmd.args) < 2 {
		return fmt.Errorf("usage: %s <name> <url>", cmd.name)
	}

	name := cmd.args[0]
	url := cmd.args[1]

	// Create new feed
	now := time.Now().UTC()
	feedParams := database.CreateFeedParams{
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
		Name:      name,
		Url:       url,
		UserID:    user.ID,
	}

	feed, err := s.db.CreateFeed(context.Background(), feedParams)
	if err != nil {
		return fmt.Errorf("couldn't create feed: %w", err)
	}

	// Automatically create a feed follow record for the user who created the feed
	followParams := database.CreateFeedFollowParams{
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
		UserID:    user.ID,
		FeedID:    feed.ID,
	}

	_, err = s.db.CreateFeedFollow(context.Background(), followParams)
	if err != nil {
		return fmt.Errorf("couldn't create feed follow: %w", err)
	}

	fmt.Printf("Feed created successfully!\n")
	fmt.Printf("Feed data: %+v\n", feed)
	return nil
}

// handlerFeeds handles the feeds command to list all feeds
func handlerFeeds(s *state, cmd command) error {
	feeds, err := s.db.GetFeeds(context.Background())
	if err != nil {
		return fmt.Errorf("couldn't get feeds: %w", err)
	}

	if len(feeds) == 0 {
		fmt.Println("No feeds found.")
		return nil
	}

	for _, feed := range feeds {
		fmt.Printf("* %s (%s) - %s\n", feed.FeedName, feed.FeedUrl, feed.UserName)
	}

	return nil
}

// handlerFollow handles the follow command to follow existing feeds by URL
func handlerFollow(s *state, cmd command, user database.User) error {
	if len(cmd.args) == 0 {
		return fmt.Errorf("usage: %s <url>", cmd.name)
	}

	url := cmd.args[0]

	// Get feed by URL
	feed, err := s.db.GetFeedByURL(context.Background(), url)
	if err != nil {
		return fmt.Errorf("couldn't find feed with URL %s: %w", url, err)
	}

	// Create feed follow record
	now := time.Now().UTC()
	followParams := database.CreateFeedFollowParams{
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
		UserID:    user.ID,
		FeedID:    feed.ID,
	}

	followRecord, err := s.db.CreateFeedFollow(context.Background(), followParams)
	if err != nil {
		return fmt.Errorf("couldn't follow feed: %w", err)
	}

	fmt.Printf("Following %s by %s\n", followRecord.FeedName, followRecord.UserName)
	return nil
}

// handlerFollowing handles the following command to list feeds current user is following
func handlerFollowing(s *state, cmd command, user database.User) error {
	// Get feed follows for user
	follows, err := s.db.GetFeedFollowsForUser(context.Background(), user.ID)
	if err != nil {
		return fmt.Errorf("couldn't get feed follows: %w", err)
	}

	if len(follows) == 0 {
		fmt.Println("You are not following any feeds.")
		return nil
	}

	fmt.Printf("Following feeds for %s:\n", user.Name)
	for _, follow := range follows {
		fmt.Printf("* %s\n", follow.FeedName)
	}

	return nil
}

// handlerUnfollow allows a user to unfollow a feed by its URL
func handlerUnfollow(s *state, cmd command, user database.User) error {
	if len(cmd.args) < 1 {
		return fmt.Errorf("usage: unfollow <feed-url>")
	}

	feedURL := cmd.args[0]
	feed, err := s.db.GetFeedByURL(context.Background(), feedURL)
	if err != nil {
		return fmt.Errorf("could not find feed with URL %s: %w", feedURL, err)
	}

	// Adjusted to handle two return values from DeleteFeedFollowByUserAndFeed
	rowsAffected, err := s.db.DeleteFeedFollowByUserAndFeed(context.Background(), database.DeleteFeedFollowByUserAndFeedParams{
		UserID: user.ID,
		FeedID: feed.ID,
	})
	if err != nil {
		return fmt.Errorf("could not unfollow feed: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("no feed follow record found to delete")
	}

	fmt.Printf("Successfully unfollowed feed: %s\n", feed.Name)
	return nil
}

// handlerBrowse supports pagination, sorting, and optional feed filtering
func handlerBrowse(s *state, cmd command, user database.User) error {
	limit := 2
	offset := 0
	sortBy := "published_at"
	order := "desc"
	feedFilter := ""

	if len(cmd.args) > 0 {
		parsedLimit, err := strconv.Atoi(cmd.args[0])
		if err != nil {
			return fmt.Errorf("invalid limit: %v", err)
		}
		limit = parsedLimit
	}
	if len(cmd.args) > 1 {
		parsedOffset, err := strconv.Atoi(cmd.args[1])
		if err != nil {
			return fmt.Errorf("invalid offset: %v", err)
		}
		offset = parsedOffset
	}
	if len(cmd.args) > 2 {
		sortBy = strings.ToLower(cmd.args[2])
	}
	if len(cmd.args) > 3 {
		order = strings.ToLower(cmd.args[3])
		if order != "asc" && order != "desc" {
			return fmt.Errorf("invalid order: must be asc or desc")
		}
	}
	if len(cmd.args) > 4 {
		feedFilter = cmd.args[4]
	}

	posts, err := s.db.GetPostsForUserPaginated(context.Background(), database.GetPostsForUserPaginatedParams{
		UserID: user.ID,
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return fmt.Errorf("error fetching posts: %v", err)
	}

	if feedFilter != "" {
		filtered := make([]database.Post, 0, len(posts))
		for _, post := range posts {
			if post.FeedID.String() == feedFilter {
				filtered = append(filtered, post)
			}
		}
		posts = filtered
	}

	switch sortBy {
	case "title":
		sort.SliceStable(posts, func(i, j int) bool {
			return strings.ToLower(posts[i].Title) < strings.ToLower(posts[j].Title)
		})
	case "published_at", "published":
		sort.SliceStable(posts, func(i, j int) bool {
			left := posts[i].PublishedAt.Time
			if !posts[i].PublishedAt.Valid {
				left = posts[i].CreatedAt
			}
			right := posts[j].PublishedAt.Time
			if !posts[j].PublishedAt.Valid {
				right = posts[j].CreatedAt
			}
			return left.Before(right)
		})
	default:
		return fmt.Errorf("unsupported sort column: %s", sortBy)
	}

	if order == "desc" {
		for i, j := 0, len(posts)-1; i < j; i, j = i+1, j-1 {
			posts[i], posts[j] = posts[j], posts[i]
		}
	}

	for _, post := range posts {
		publishedAt := post.CreatedAt
		if post.PublishedAt.Valid {
			publishedAt = post.PublishedAt.Time
		}
		description := ""
		if post.Description.Valid {
			description = post.Description.String
		}
		fmt.Printf("Title: %s\nURL: %s\nPublished At: %s\nDescription: %s\nFeed ID: %s\n\n",
			post.Title,
			post.Url,
			publishedAt.Format(time.RFC1123),
			description,
			post.FeedID,
		)
	}

	return nil
}

// handlerSearch allows users to perform fuzzy searches on posts
func handlerSearch(s *state, cmd command, user database.User) error {
	if len(cmd.args) < 1 {
		return fmt.Errorf("usage: search <query>")
	}

	query := cmd.args[0]
	posts, err := s.db.SearchPosts(context.Background(), database.SearchPostsParams{
		UserID: user.ID,
		Title:  fmt.Sprintf("%%%s%%", query),
	})
	if err != nil {
		return fmt.Errorf("error searching posts: %v", err)
	}

	for _, post := range posts {
		publishedAt := post.CreatedAt
		if post.PublishedAt.Valid {
			publishedAt = post.PublishedAt.Time
		}
		fmt.Printf("Title: %s\nURL: %s\nPublished At: %s\n\n", post.Title, post.Url, publishedAt.Format(time.RFC1123))
	}

	return nil
}

// handlerBookmark allows users to bookmark a post
func handlerBookmark(s *state, cmd command, user database.User) error {
	if len(cmd.args) < 1 {
		return fmt.Errorf("usage: bookmark <post-id>")
	}

	postID := cmd.args[0]
	parsedPostID, err := uuid.Parse(postID)
	if err != nil {
		return fmt.Errorf("invalid post ID: %w", err)
	}

	err = s.db.BookmarkPost(context.Background(), database.BookmarkPostParams{
		UserID: user.ID,
		PostID: parsedPostID,
	})
	if err != nil {
		return fmt.Errorf("error bookmarking post: %v", err)
	}

	fmt.Printf("Post %s bookmarked successfully!\n", postID)
	return nil
}

// handlerTUI launches the terminal user interface for viewing posts
func handlerTUI(s *state, cmd command, user database.User) error {
	posts, err := s.db.GetPostsForUser(context.Background(), database.GetPostsForUserParams{
		UserID: user.ID,
		Limit:  100, // Fetch up to 100 posts for the TUI
	})
	if err != nil {
		return fmt.Errorf("error fetching posts: %v", err)
	}

	formattedPosts := make([]tui.Post, len(posts))
	for i, post := range posts {
		formattedPosts[i] = tui.Post{
			Title: post.Title,
			URL:   post.Url,
		}
	}

	tui.StartTUI(formattedPosts)
	return nil
}

// handlerAPI starts the HTTP API server
func handlerAPI(s *state, cmd command) error {
	fmt.Println("Starting HTTP API server on port 8080...")
	api.StartAPI()
	return nil
}

// scrapeFeeds fetches the next feed, marks it as fetched, and prints post titles
func scrapeFeeds(s *state) {
	feed, err := s.db.GetNextFeedToFetch(context.Background())
	if err != nil {
		log.Printf("error fetching next feed: %v", err)
		return
	}

	if err := s.db.MarkFeedFetched(context.Background(), feed.ID); err != nil {
		log.Printf("error marking feed as fetched: %v", err)
		return
	}

	fmt.Printf("Fetching feed: %s (%s)\n", feed.Name, feed.Url)
	rssFeed, err := fetchFeed(context.Background(), feed.Url)
	if err != nil {
		log.Printf("error fetching feed URL %s: %v", feed.Url, err)
		return
	}

	for _, item := range rssFeed.Channel.Item {
		description := sql.NullString{String: strings.TrimSpace(item.Description), Valid: strings.TrimSpace(item.Description) != ""}
		pubTime, ok := parsePublished(item.PubDate)
		publishedAt := sql.NullTime{}
		if ok {
			publishedAt = sql.NullTime{Time: pubTime, Valid: true}
		}

		postParams := database.CreatePostParams{
			ID:          uuid.New(),
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
			Title:       strings.TrimSpace(item.Title),
			Url:         strings.TrimSpace(item.Link),
			Description: description,
			PublishedAt: publishedAt,
			FeedID:      feed.ID,
		}

		if err := s.db.CreatePost(context.Background(), postParams); err != nil {
			log.Printf("error saving post %s: %v", item.Link, err)
		}
	}
}

var publishedLayouts = []string{
	time.RFC1123Z,
	time.RFC1123,
	time.RFC822Z,
	time.RFC822,
	time.RFC3339,
	time.RubyDate,
	"Mon, 02 Jan 2006 15:04:05 -0700",
}

func parsePublished(raw string) (time.Time, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return time.Time{}, false
	}
	for _, layout := range publishedLayouts {
		if parsed, err := time.Parse(layout, trimmed); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

// handlerAggService keeps the agg command running and restarts it on failure
func handlerAggService(s *state, cmd command) error {
	if len(cmd.args) < 1 {
		return fmt.Errorf("usage: aggservice <time_between_reqs>")
	}

	timeArg := cmd.args[0]
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("unable to determine executable path: %w", err)
	}

	restartDelay := 5 * time.Second
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigs)

	log.Printf("Starting agg service manager with interval %s", timeArg)

	remainingArgs := []string{"agg", timeArg}

	for {
		cmdCtx, cancel := context.WithCancel(context.Background())
		aggCmd := exec.CommandContext(cmdCtx, execPath, remainingArgs...)
		aggCmd.Stdout = os.Stdout
		aggCmd.Stderr = os.Stderr
		aggCmd.Env = os.Environ()

		errCh := make(chan error, 1)
		go func() {
			errCh <- aggCmd.Run()
		}()

		select {
		case sig := <-sigs:
			log.Printf("Received signal %s, shutting down agg service", sig)
			cancel()
			if aggCmd.Process != nil {
				_ = aggCmd.Process.Signal(sig)
			}
			return nil
		case runErr := <-errCh:
			cancel()
			if runErr != nil {
				log.Printf("agg command exited with error: %v", runErr)
			} else {
				log.Printf("agg command exited cleanly")
			}
		}

		select {
		case sig := <-sigs:
			log.Printf("Received signal %s during restart window, exiting", sig)
			return nil
		case <-time.After(restartDelay):
			log.Printf("Restarting agg command after %s", restartDelay)
		}
	}
}

func main() {
	// Read the config file
	cfg, err := config.Read()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading config: %v\n", err)
		os.Exit(1)
	}

	// Open database connection
	db, err := sql.Open("postgres", cfg.DbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Create database queries instance
	dbQueries := database.New(db)

	// Create state with config and database
	programState := &state{
		db:  dbQueries,
		cfg: &cfg,
	}

	// Create commands struct with initialized map
	cmds := &commands{
		handlers: make(map[string]func(*state, command) error),
	}

	// Register command handlers
	cmds.register("register", handlerRegister)
	cmds.register("login", handlerLogin)
	cmds.register("reset", handlerReset)
	cmds.register("users", handlerUsers)
	cmds.register("agg", handlerAgg)
	cmds.register("addfeed", middlewareLoggedIn(handlerAddfeed))
	cmds.register("feeds", handlerFeeds)
	cmds.register("follow", middlewareLoggedIn(handlerFollow))
	cmds.register("following", middlewareLoggedIn(handlerFollowing))
	cmds.register("unfollow", middlewareLoggedIn(handlerUnfollow))
	cmds.register("browse", middlewareLoggedIn(handlerBrowse))
	cmds.register("search", middlewareLoggedIn(handlerSearch))
	cmds.register("bookmark", middlewareLoggedIn(handlerBookmark))
	cmds.register("tui", middlewareLoggedIn(handlerTUI))
	cmds.register("api", handlerAPI)
	cmds.register("aggservice", handlerAggService)

	// Get command-line arguments
	args := os.Args
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <command> [args...]\n", args[0])
		os.Exit(1)
	}

	// Parse command name and arguments
	cmdName := args[1]
	cmdArgs := []string{}
	if len(args) > 2 {
		cmdArgs = args[2:]
	}

	// Create command instance
	cmd := command{
		name: cmdName,
		args: cmdArgs,
	}

	// Run the command
	err = cmds.run(programState, cmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
