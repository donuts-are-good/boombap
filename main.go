package main

import (
	"bufio"
	"io"
	"net/http"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/faiface/beep"
	"github.com/faiface/beep/speaker"
	"github.com/faiface/beep/vorbis"
	"github.com/hajimehoshi/go-mp3"
)

var channels = []string{
	"https://somafm.com/groovesalad256.pls",
	"https://somafm.com/dronezone256.pls",
	"https://somafm.com/spacestation.pls",
	"https://somafm.com/deepspaceone.pls",
	"https://somafm.com/secretagent.pls",
	"https://somafm.com/gsclassic.pls",
	"https://somafm.com/u80s256.pls",
	"https://somafm.com/synphaera256.pls",
	"https://somafm.com/beatblender.pls",
	"https://somafm.com/defcon256.pls",
	"https://somafm.com/thetrip.pls",
	"https://somafm.com/sonicuniverse256.pls",
	"https://somafm.com/7soul.pls",
	"https://somafm.com/cliqhop256.pls",
	"https://somafm.com/illstreet.pls",
	"https://somafm.com/fluid.pls",
	"https://somafm.com/reggae256.pls",
	"https://somafm.com/missioncontrol.pls",
	"https://somafm.com/darkzone256.pls",
	"https://somafm.com/dubstep256.pls",
	"https://somafm.com/sf1033.pls",
	"https://somafm.com/vaporwaves.pls",
	"https://somafm.com/metal.pls",
	"https://somafm.com/specials.pls",
	"https://somafm.com/n5md.pls",
	"https://somafm.com/scanner.pls",
	"https://somafm.com/sfinsf.pls",
}

var selectedChannel widget.ListItemID
var loadingIndicator *widget.ProgressBarInfinite

func main() {
	a := app.New()

	w := a.NewWindow("Internet Radio")

	w.Resize(fyne.NewSize(300, 600))
	playBtn := widget.NewButton("Play", nil)
	pauseBtn := widget.NewButton("Pause", nil)
	backBtn := widget.NewButton("Back", nil)
	nextBtn := widget.NewButton("Next", nil)

	controlBox := container.NewHBox(playBtn, pauseBtn, nextBtn, backBtn)
	channelList := widget.NewList(
		func() int { return len(channels) },
		func() fyne.CanvasObject {
			return widget.NewLabel("Channel")
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			o.(*widget.Label).SetText(channels[i])
		},
	)

	channelList.OnSelected = func(id widget.ListItemID) {
		selectedChannel = id
	}

	playBtn.OnTapped = func() {
		playStream(channels[selectedChannel])
	}

	pauseBtn.OnTapped = func() {
		speaker.Clear()
	}

	nextBtn.OnTapped = func() {
		selectedChannel = (selectedChannel + 1) % len(channels)
		channelList.Select(selectedChannel)
		playStream(channels[selectedChannel])
	}

	backBtn.OnTapped = func() {
		selectedChannel = (selectedChannel - 1 + len(channels)) % len(channels)
		channelList.Select(selectedChannel)
		playStream(channels[selectedChannel])
	}

	loadingIndicator = widget.NewProgressBarInfinite()
	loadingIndicator.Hide()

	content := container.NewBorder(controlBox, nil, nil, nil, channelList, loadingIndicator)
	w.SetContent(content)
	w.ShowAndRun()
}

func startLoading() {
	loadingIndicator.Show()
}

func stopLoading() {
	loadingIndicator.Hide()
}

func playStream(url string) {
	startLoading()
	defer stopLoading()

	plsResp, err := http.Get(url)
	if err != nil {
		return
	}
	defer plsResp.Body.Close()

	playlist, err := parsePls(plsResp.Body)
	if err != nil {
		return
	}
	streamURL := playlist[0]

	resp, err := http.Get(streamURL)
	if err != nil {
		return
	}

	ext := resp.Header.Get("Content-Type")
	var stream beep.Streamer
	var format beep.Format

	switch ext {
	case "audio/ogg":
		stream, format, _ = vorbis.Decode(resp.Body)
	case "audio/mpeg":
		decoder, err := mp3.NewDecoder(resp.Body)
		if err != nil {
			return
		}
		stream = beep.StreamerFunc(func(samples [][2]float64) (n int, ok bool) {
			data := make([]byte, 4*len(samples))
			n, err := decoder.Read(data)
			if err != nil {
				return 0, false
			}
			for i := 0; i < n/4; i++ {
				samples[i][0] = float64(int16(data[4*i+1])<<8|int16(data[4*i])) / 32768
				samples[i][1] = float64(int16(data[4*i+3])<<8|int16(data[4*i+2])) / 32768
			}
			return n / 4, true
		})
		format = beep.Format{
			SampleRate:  beep.SampleRate(decoder.SampleRate()),
			NumChannels: 2,
			Precision:   2,
		}
	default:
		return
	}

	speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))

	done := make(chan struct{})
	speaker.Play(beep.Seq(stream, beep.Callback(func() {
		close(done)
	})))

	go func() {
		<-done
		resp.Body.Close()
	}()
}

func parsePls(r io.Reader) ([]string, error) {
	scanner := bufio.NewScanner(r)
	var urls []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "File") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				urls = append(urls, parts[1])
			}
		}
	}
	return urls, scanner.Err()
}
