package main

import (
	"embed"
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"io/ioutil"
	"log"
	"math"
	"math/rand"
	"os"

	"github.com/google/uuid"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	logging "github.com/tsujio/game-logging-server/client"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

const (
	gameName                      = "birdman"
	screenWidth                   = 640
	screenHeight                  = 480
	birdmanHeight                 = 100
	birdmanWidth                  = 100
	birdHeight                    = 100
	birdWidth                     = 100
	birdmanAndBirdCollisionRadius = 50
	initialBirdmanPosY            = screenHeight / 3
	cliffWidth                    = 100
	titleFontSize                 = regularFontSize * 1.5
	regularFontSize               = 24
	smallFontSize                 = regularFontSize / 2
)

//go:embed resources/*.ttf resources/*.png resources/*.dat
var resources embed.FS

var (
	seaImg                            = loadImage("resources/sea.png")
	cliffImg                          = loadImage("resources/cliff.png")
	backgroundImg                     = loadImage("resources/background.png")
	birdmanImg                        = loadImage("resources/birdman.png")
	birdImg                           = loadImage("resources/bird.png")
	titleFont, regularFont, smallFont = loadFont("resources/PressStart2P-Regular.ttf")
	audioContext                      = audio.NewContext(48000)
	damageAudioData                   = loadAudioData("resources/魔王魂  レトロ22.mp3.dat", audioContext)
	gameOverAudioData                 = loadAudioData("resources/魔王魂  レトロ12.mp3.dat", audioContext)
	flyingAudioData                   = loadAudioData("resources/魔王魂 効果音 羽音01.mp3.dat", audioContext)
)

func loadImage(name string) *ebiten.Image {
	f, err := resources.Open(name)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		log.Fatal(err)
	}
	return ebiten.NewImageFromImage(img)
}

func loadFont(name string) (titleFont, regularFont, smallFont font.Face) {
	f, err := resources.Open(name)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	fontData, err := ioutil.ReadAll(f)
	if err != nil {
		log.Fatal(err)
	}
	tt, err := opentype.Parse(fontData)
	if err != nil {
		log.Fatal(err)
	}

	const dpi = 72
	titleFont, err = opentype.NewFace(tt, &opentype.FaceOptions{
		Size:    titleFontSize,
		DPI:     dpi,
		Hinting: font.HintingFull,
	})
	if err != nil {
		log.Fatal(err)
	}
	regularFont, err = opentype.NewFace(tt, &opentype.FaceOptions{
		Size:    regularFontSize,
		DPI:     dpi,
		Hinting: font.HintingFull,
	})
	if err != nil {
		log.Fatal(err)
	}
	smallFont, err = opentype.NewFace(tt, &opentype.FaceOptions{
		Size:    smallFontSize,
		DPI:     dpi,
		Hinting: font.HintingFull,
	})
	if err != nil {
		log.Fatal(err)
	}

	return
}

func loadAudioData(name string, audioContext *audio.Context) []byte {
	f, err := resources.Open(name)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	data, err := ioutil.ReadAll(f)
	if err != nil {
		log.Fatal(err)
	}

	return data
}

func formatIntComma(n int) string {
	s := fmt.Sprintf("%d", n)
	ret := ""
	for i, c := range s {
		ret += string(c)
		if i+1 < len(s) && (len(s)-i-1)%3 == 0 {
			ret += ","
		}
	}
	return ret
}

type BirdmanState int

const (
	StateRunning BirdmanState = iota
	StateFlying
	StateDamaged
)

type Birdman struct {
	img          *ebiten.Image
	state        BirdmanState
	x, y         int
	vy           int
	damagedCount int
	damagedTicks int
}

func (b *Birdman) Draw(screen *ebiten.Image, game *Game) {
	switch b.state {
	case StateRunning:
		img := b.img.SubImage(image.Rect(
			0,
			0,
			birdmanWidth,
			birdmanHeight,
		)).(*ebiten.Image)
		x := float64(b.x-game.cameraX) - float64(birdmanWidth)/2
		y := float64(b.y) - float64(birdmanHeight)/2
		opt := &ebiten.DrawImageOptions{}
		opt.GeoM.Translate(x, y)
		screen.DrawImage(img, opt)
	case StateFlying:
		frameIndex := b.x / 10 % 2
		img := b.img.SubImage(image.Rect(
			birdmanWidth*frameIndex,
			0,
			birdmanWidth*(frameIndex+1),
			birdmanHeight,
		)).(*ebiten.Image)
		x := float64(b.x-game.cameraX) - float64(birdmanWidth)/2
		y := float64(b.y) - float64(birdmanHeight)/2
		opt := &ebiten.DrawImageOptions{}
		opt.GeoM.Translate(x, y)
		screen.DrawImage(img, opt)
	case StateDamaged:
		img := b.img.SubImage(image.Rect(
			birdmanWidth,
			0,
			birdmanWidth*2,
			birdmanHeight,
		)).(*ebiten.Image)
		x := float64(b.x - game.cameraX)
		y := float64(b.y)
		opt := &ebiten.DrawImageOptions{}
		opt.GeoM.Translate(-float64(birdmanWidth)/2, -float64(birdmanHeight)/2)
		opt.GeoM.Rotate(float64(b.damagedTicks) / 3)
		opt.GeoM.Translate(x, y)
		screen.DrawImage(img, opt)
	}
}

type Bird struct {
	img  *ebiten.Image
	x, y int
}

func (b *Bird) Draw(screen *ebiten.Image, game *Game) {
	frameIndex := b.x / 10 % 2
	img := b.img.SubImage(image.Rect(
		birdWidth*frameIndex,
		0,
		birdWidth*(frameIndex+1),
		birdHeight,
	)).(*ebiten.Image)
	x := float64(b.x-game.cameraX) - float64(birdWidth)/2
	y := float64(b.y) - float64(birdHeight)/2
	opt := &ebiten.DrawImageOptions{}
	opt.GeoM.Translate(x, y)
	screen.DrawImage(img, opt)
}

type Mode int

const (
	ModeTitle Mode = iota
	ModeGame
	ModeGameOver
)

type Game struct {
	playID           string
	initializeCount  int
	mode             Mode
	birdman          *Birdman
	birds            []Bird
	cameraX, cameraY int
}

func (g *Game) isJustTapped() bool {
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return true
	}
	if touchIDs := inpututil.JustPressedTouchIDs(); len(touchIDs) > 0 {
		return true
	}
	return false
}

func (g *Game) Update() error {
	birdman := g.birdman

	switch g.mode {
	case ModeTitle:
		if g.isJustTapped() {
			logging.LogAsync(gameName, map[string]interface{}{
				"play_id": g.playID,
				"action":  "start_game",
			})

			g.mode = ModeGame
		}
	case ModeGame:
		switch birdman.state {
		case StateRunning:
			birdman.x += 1
			if birdman.x >= 0 {
				birdman.state = StateFlying
			}
		case StateFlying:
			// Camera move
			g.cameraX += 1

			// Birds appearance
			if birdman.x%200 == 0 {
				b := Bird{
					img: birdImg,
					x:   birdman.x + screenWidth,
					y:   50 + rand.Int()%(screenHeight-100),
				}
				g.birds = append(g.birds, b)
			}

			// Birds move
			var newBirds []Bird
			for i := 0; i < len(g.birds); i++ {
				g.birds[i].x -= 1
				if g.birds[i].x+birdWidth > g.cameraX {
					newBirds = append(newBirds, g.birds[i])
				}
			}
			g.birds = newBirds

			// User input
			if g.isJustTapped() {
				var ay int
				if birdman.x < 1000 {
					ay = -20
				} else if birdman.x < 2000 {
					ay = -15
				} else if birdman.x < 3000 {
					ay = -10
				} else if birdman.x < 4000 {
					ay = -7
				} else {
					ay = -5
				}
				ay /= birdman.damagedCount + 1
				birdman.vy += ay

				audio.NewPlayerFromBytes(audioContext, flyingAudioData).Play()
			}

			// Birdman gravity
			birdman.vy += 1
			if birdman.vy > 5 {
				birdman.vy = 5
			}

			// Birdman move
			birdman.x += 1
			birdman.y += birdman.vy

			// Birdman too high
			if birdman.y < 0 {
				birdman.damagedCount += 1
				birdman.state = StateDamaged

				audio.NewPlayerFromBytes(audioContext, damageAudioData).Play()
			}

			// Birdman and birds collision
			for i := 0; i < len(g.birds); i++ {
				if math.Pow(float64(birdman.x-g.birds[i].x), 2)+math.Pow(float64(birdman.y-g.birds[i].y), 2) <
					math.Pow(birdmanAndBirdCollisionRadius, 2) {
					birdman.damagedCount += 1
					birdman.state = StateDamaged

					audio.NewPlayerFromBytes(audioContext, damageAudioData).Play()

					break
				}
			}

			// Birdman fall
			if birdman.y > screenHeight {
				logging.LogAsync(gameName, map[string]interface{}{
					"play_id":       g.playID,
					"action":        "game_over",
					"x":             birdman.x,
					"damaged_count": birdman.damagedCount,
				})

				g.mode = ModeGameOver

				audio.NewPlayerFromBytes(audioContext, gameOverAudioData).Play()
			}
		case StateDamaged:
			// Birds move
			for i := 0; i < len(g.birds); i++ {
				g.birds[i].x -= 1
			}

			// Birdman move
			birdman.damagedTicks += 1
			birdman.vy = 0
			birdman.y += 1

			if birdman.y > screenHeight {
				logging.LogAsync(gameName, map[string]interface{}{
					"play_id":       g.playID,
					"action":        "game_over",
					"x":             birdman.x,
					"damaged_count": birdman.damagedCount,
				})

				g.mode = ModeGameOver

				audio.NewPlayerFromBytes(audioContext, gameOverAudioData).Play()
			}

			if birdman.damagedTicks%ebiten.MaxTPS() == 0 {
				birdman.damagedTicks = 0
				birdman.state = StateFlying
			}
		}
	case ModeGameOver:
		if g.isJustTapped() {
			g.initialize()
		}
	}

	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	backgroundImgWidth, backgroundImgHeight := backgroundImg.Size()
	seaImgWidth, seaImgHeight := seaImg.Size()

	// Background sky
	for i := -1; i < screenWidth/backgroundImgWidth+2; i++ {
		backgroundImgOpt := &ebiten.DrawImageOptions{}
		backgroundImgOpt.GeoM.Scale(
			1.0,
			float64(screenHeight-seaImgHeight)/float64(backgroundImgHeight),
		)
		backgroundImgOpt.GeoM.Translate(
			float64(i*backgroundImgWidth-g.cameraX%backgroundImgWidth),
			0,
		)
		screen.DrawImage(backgroundImg, backgroundImgOpt)
	}

	// Sea
	for i := -1; i < screenWidth/seaImgWidth+2; i++ {
		seaImgOpt := &ebiten.DrawImageOptions{}
		seaImgOpt.GeoM.Translate(
			float64(i*seaImgWidth-g.cameraX%seaImgWidth),
			float64(screenHeight-seaImgHeight),
		)
		screen.DrawImage(seaImg, seaImgOpt)
	}

	// Cliff
	cliffImgWidth, _ := cliffImg.Size()
	cliffImgOpt := &ebiten.DrawImageOptions{}
	cliffImgOpt.GeoM.Scale(cliffWidth/float64(cliffImgWidth), 1.0)
	cliffImgOpt.GeoM.Translate(
		float64(-cliffWidth-g.cameraX),
		initialBirdmanPosY+birdmanHeight/3,
	)
	screen.DrawImage(cliffImg, cliffImgOpt)

	// Birdman
	g.birdman.Draw(screen, g)

	// Birds
	for i := 0; i < len(g.birds); i++ {
		g.birds[i].Draw(screen, g)
	}

	// Texts
	record := g.birdman.x / 10
	switch g.mode {
	case ModeTitle:
		titleText := "BIRDMAN CHALLENGE"
		text.Draw(screen, titleText, titleFont, screenWidth/2-len(titleText)*titleFontSize/2, 90, color.White)
		descriptionText := "CLICK TO START"
		text.Draw(screen, descriptionText, regularFont, screenWidth/2-len(descriptionText)*regularFontSize/2, 170, color.White)

		licenseTexts := []string{"PHOTO: OITA-SHI (FIND/47)", "FONT: Press Start 2P by CodeMan38", "SOUND: MaouDamashii"}
		for i, s := range licenseTexts {
			text.Draw(screen, s, smallFont, screenWidth/2-len(s)*smallFontSize/2, int(420+float32(i)*smallFontSize*1.7), color.White)
		}
	case ModeGame:
		recordText := fmt.Sprintf("%sm", formatIntComma(record))
		text.Draw(screen, recordText, smallFont, 24, 24, color.White)
	case ModeGameOver:
		const gameOverText = "GAME OVER"
		text.Draw(screen, gameOverText, titleFont, screenWidth/2-len(gameOverText)*titleFontSize/2, 180, color.White)
		recordText := []string{"YOUR RECORD IS", fmt.Sprintf("%sm!", formatIntComma(record))}
		for i, s := range recordText {
			text.Draw(screen, s, regularFont, screenWidth/2-len(s)*regularFontSize/2, 250+i*(regularFontSize*2), color.White)
		}
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenWidth, screenHeight
}

func (g *Game) initialize() {
	g.initializeCount++

	logging.LogAsync(gameName, map[string]interface{}{
		"play_id": g.playID,
		"action":  "initialize",
		"count":   g.initializeCount,
	})

	g.mode = ModeTitle
	g.cameraX = -100
	g.cameraY = 0

	birdman := &Birdman{
		img:          birdmanImg,
		state:        StateRunning,
		x:            -60,
		y:            initialBirdmanPosY,
		vy:           0,
		damagedCount: 0,
		damagedTicks: 0,
	}
	g.birdman = birdman

	g.birds = nil
}

func main() {
	if os.Getenv("GAME_LOGGING") != "1" {
		logging.Disable()
	}

	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetWindowTitle("Birdman")

	playIDObj, err := uuid.NewRandom()
	var playID string
	if err != nil {
		playID = "?"
	} else {
		playID = playIDObj.String()

	}
	game := &Game{
		playID:          playID,
		initializeCount: 0,
	}
	game.initialize()

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
