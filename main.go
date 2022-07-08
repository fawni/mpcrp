package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/logrusorgru/aurora/v3"
	PTN "github.com/middelink/go-parse-torrent-name"
	"github.com/spf13/cobra"
	"github.com/x6r/rp"
	"github.com/x6r/rp/rpc"
)

type state int8

type playback struct {
	file           string
	state          state
	position       int
	duration       int
	durationstring string
	version        string
}

type Media struct {
	Category string `json:"type,omitempty"`
	Title    string `json:"title,omitempty"`
	Poster   string `json:"poster,omitempty"`
}

const (
	idling state = iota - 1
	stopped
	paused
	playing
)

var (
	c    *rpc.Client
	pb   playback
	file string

	id   uint64
	port uint16
	raw  bool

	cmd = &cobra.Command{
		Use:   "mpcrp",
		Short: "mpcrp is a cross-platform discord rich presence integration for mpc-hc",
		RunE: func(cmd *cobra.Command, args []string) error {
			return start()
		},
	}
)

func init() {
	cmd.PersistentFlags().Uint64VarP(&id, "id", "i", 955267481772130384, "app id providing rich presence assets")
	cmd.PersistentFlags().Uint16VarP(&port, "port", "p", 13579, "port to connect to")
	cmd.PersistentFlags().BoolVarP(&raw, "raw", "r", false, "display only the filename without fanart")

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	go func() {
		<-ch
		fmt.Println(aurora.Red("User interuptted! Exiting..."))
		os.Exit(0)
	}()
}

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Println(aurora.Red(err))
		os.Exit(1)
	}

	if c != nil {
		c.Logout()
	}
}

func start() error {
	var err error
	c, err = rp.NewClient(fmt.Sprintf("%d", id))
	if err != nil {
		return fmt.Errorf("Could not connect to discord rich presence client.")
	}

	go forever()
	fmt.Println(aurora.Green(fmt.Sprintf("Listening on port: %d!", port)))
	select {}
}

func forever() {
	for {
		if err := readVariables(); err != nil {
			if err := c.ResetActivity(); err != nil {
				fmt.Println(aurora.Red(err))
			}
			c.Logged = false
			continue
		} else if !c.Logged {
			if err := c.Login(); err != nil {
				fmt.Println(aurora.Red(err))
			}
		}
		updatePayload()
		time.Sleep(time.Second)
	}
}

func readVariables() error {
	uri := fmt.Sprintf("http://localhost:%d/variables.html", port)
	res, err := http.Get(uri)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("status code error: %d %s", res.StatusCode, res.Status)
	}

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return err
	}

	doc.Find(".page-variables").Each(func(_ int, s *goquery.Selection) {
		istate, _ := strconv.Atoi(s.Find("#state").Text())
		state := state(istate)
		position, _ := strconv.Atoi(s.Find("#position").Text())
		duration, _ := strconv.Atoi(s.Find("#duration").Text())
		pb = playback{
			file:           s.Find("#file").Text(),
			state:          state,
			position:       position,
			duration:       duration,
			durationstring: s.Find("#durationstring").Text(),
			version:        s.Find("#version").Text(),
		}
	})

	return nil
}

func updatePayload() {
	var m Media
	ptn, err := PTN.Parse(pb.file)
	if err != nil {
		fmt.Println(aurora.Red(err))
	}

	if file != pb.file {
		m = setInfo(ptn)
	}

	activity := &rpc.Activity{
		Details:    pb.file,
		LargeImage: "mpc-hc",
		LargeText:  "mpc-hc " + pb.version,
		SmallText:  pb.durationstring,
	}

	if m.Title != "" && !raw {
		activity.Details = ptn.Title
		activity.LargeImage = m.Poster
		activity.LargeText = ptn.Title

		switch m.Category {
		case "TV Show":
			activity.State = fmt.Sprintf("S%d:E%d", ptn.Season, ptn.Episode)
		case "Movie":
			activity.State = m.Category
		}
	}

	position, duration := pb.position, pb.duration
	remaining, _ := time.ParseDuration(strconv.Itoa(duration-position) + "ms")
	start := time.Now()
	end := start.Add(remaining)

	switch pb.state {
	case paused:
		activity.SmallImage = "pause"
	case stopped:
		activity.SmallImage = "stop"
	case playing:
		activity.SmallImage = "play"
		activity.Timestamps = &rpc.Timestamps{
			Start: &start,
			End:   &end,
		}
	case idling:
		activity = &rpc.Activity{
			Details:    "idling",
			LargeImage: "mpc-hc",
			LargeText:  "mpc-hc " + pb.version,
		}
	}

	if err := c.SetActivity(activity); err != nil {
		fmt.Println(aurora.Red(err))
	}
}

func setInfo(ptn *PTN.TorrentInfo) Media {
	uri := "https://fanart.tv/api/search.php?section=everything&s=" + url.QueryEscape(fmt.Sprintf("%s %d", ptn.Title, ptn.Year))
	res, err := http.Get(uri)
	if err != nil {
		fmt.Println(aurora.Red(err))
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Println(aurora.Red(err))
	}
	var medias []Media
	if err := json.Unmarshal(body, &medias); err != nil {
		fmt.Println(aurora.Red(err))
	}

	for _, media := range medias {
		if (media.Category == "TV Show" || media.Category == "Movie") && media.Poster != "" {
			return media
		}
	}

	return Media{}
}
