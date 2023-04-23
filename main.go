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
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/faiface/beep"
	"github.com/faiface/beep/effects"
	"github.com/faiface/beep/speaker"
	"github.com/faiface/beep/vorbis"
	"github.com/hajimehoshi/go-mp3"
)

type station struct {
	Name        string
	URL         string
	Description string
}

var volumeControl *effects.Volume
var selectedChannel widget.ListItemID
var loadingIndicator *widget.ProgressBarInfinite

func main() {
	a := app.New()

	w := a.NewWindow("Boombap")

	w.Resize(fyne.NewSize(250, 500))
	playBtn := widget.NewButton("Play", nil)
	pauseBtn := widget.NewButton("Pause", nil)
	backBtn := widget.NewButton("back", nil)
	nextBtn := widget.NewButton("next", nil)
	detailsBtn := widget.NewButton("Info", nil)
	volumeSlider := widget.NewSlider(0, 1)
	volumeSlider.Value = 0.5
	volumeSlider.OnChanged = func(value float64) {
		if volumeControl != nil {
			volumeControl.Volume = value
		}
	}

	controlBox := container.NewHBox(playBtn, pauseBtn, backBtn, nextBtn, detailsBtn)
	channelList := widget.NewList(
		func() int { return len(channels) },
		func() fyne.CanvasObject {
			return widget.NewLabel("Channel")
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			label := o.(*widget.Label)
			label.SetText(channels[i].Name)
		},
	)

	channelList.OnSelected = func(id widget.ListItemID) {
		selectedChannel = id
	}

	playBtn.OnTapped = func() {
		playStream(channels[selectedChannel].URL)
	}

	pauseBtn.OnTapped = func() {
		speaker.Clear()
	}

	nextBtn.OnTapped = func() {
		selectedChannel = (selectedChannel + 1) % len(channels)
		channelList.Select(selectedChannel)
		playStream(channels[selectedChannel].URL)
	}

	backBtn.OnTapped = func() {
		selectedChannel = (selectedChannel - 1 + len(channels)) % len(channels)
		channelList.Select(selectedChannel)
		playStream(channels[selectedChannel].URL)
	}

	detailsBtn.OnTapped = func() {
		selectedIndex := int(selectedChannel)
		if selectedIndex >= 0 && selectedIndex < len(channels) {
			details := widget.NewLabel(channels[selectedIndex].Description)
			details.Wrapping = fyne.TextWrapWord
			details.Resize(fyne.NewSize(245, 0))
			dialog.ShowCustom("Channel Details", "Close", details, w)
		}
	}

	loadingIndicator = widget.NewProgressBarInfinite()
	loadingIndicator.Hide()

	content := container.NewBorder(controlBox, nil, nil, nil, channelList)
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

var channels = []station{
	{"Groove Salad", "https://somafm.com/groovesalad256.pls", "A nicely chilled plate of ambient/downtempo beats and grooves."},
	{"Drone Zone", "https://somafm.com/dronezone256.pls", "Served best chilled, safe with most medications. Atmospheric textures with minimal beats."},
	{"Space Station Soma", "https://somafm.com/spacestation.pls", "Tune in, turn on, space out. Spaced-out ambient and mid-tempo electronica."},
	{"Deep Space One", "https://somafm.com/deepspaceone.pls", "Deep ambient electronic, experimental and space music. For inner and outer space exploration."},
	{"Secret Agent", "https://somafm.com/secretagent.pls", "The soundtrack for your stylish, mysterious, dangerous life. For Spies and PIs too!"},
	{"Groove Salad Classic", "https://somafm.com/gsclassic.pls", "The classic and influential downtempo electronica channel from SomaFM."},
	{"Underground 80s", "https://somafm.com/u80s256.pls", "Early 80s UK Synthpop and a bit of New Wave."},
	{"Synphaera Radio", "https://somafm.com/synphaera256.pls", "Ambient, techno and electronic music from the underground."},
	{"Beat Blender", "https://somafm.com/beatblender.pls", "A late night blend of deep-house and downtempo chill."},
	{"DEF CON Radio", "https://somafm.com/defcon256.pls", "Music for Hacking. The DEF CON Year-Round Channel."},
	{"The Trip", "https://somafm.com/thetrip.pls", "Progressive house / trance. Tip top tunes."},
	{"Sonic Universe", "https://somafm.com/sonicuniverse256.pls", "Transcending the world of jazz with eclectic, avant-garde takes on tradition."},
	{"Seven Inch Soul", "https://somafm.com/7soul.pls", "Vintage soul tracks from the original 45 RPM vinyl."},
	{"Cliqhop", "https://somafm.com/cliqhop256.pls", "Blips'n'beeps backed mostly w/beats. Intelligent Dance Music."},
	{"Illinois Street Lounge", "https://somafm.com/illstreet.pls", "Classic bachelor pad, playful exotica and vintage music of tomorrow."},
	{"Fluid", "https://somafm.com/fluid.pls", "NEW! Drown in the electronic sound of instrumental hiphop, future soul and liquid trap."},
	{"Reggae", "https://somafm.com/reggae256.pls", "Vintage Reggae and Dub"},
	{"Mission Control", "https://somafm.com/missioncontrol.pls", "Celebrating NASA and Space Explorers everywhere."},
	{"The Darkroom", "https://somafm.com/darkzone256.pls", "Indie pop and chillout tracks with an edge."},
	{"Dub Step Beyond", "https://somafm.com/dubstep256.pls", "Dubstep, Dub and Deep Bass. May damage speakers at high volume."},
	{"SF 10-33", "https://somafm.com/sf1033.pls", "Ambient music mixed with the sounds of San Francisco public safety radio traffic."},
	{"Vaporwaves", "https://somafm.com/vaporwaves.pls", "A nostalgic journey through 80s and 90s internet and computer culture."},
	{"Metal Detector", "https://somafm.com/metal.pls", "From black to doom, prog to sludge, thrash to post, stoner to crossover, punk to industrial."},
	{"Specials", "https://somafm.com/specials.pls", "A selection of special broadcasts and one-time events."},
	{"n5MD Radio", "https://somafm.com/n5md.pls", "Ambient and IDM music from the n5MD label."},
	{"Scanner: Dark Ambient", "https://somafm.com/scanner.pls", "Dark ambient music for the mind's eye."},
	{"SF in SF", "https://somafm.com/sfinsf.pls", "Science fiction, fantasy, and horror from the SF in SF reading series."},
}
