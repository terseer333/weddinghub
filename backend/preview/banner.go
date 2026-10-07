package preview

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
	"weddinghub/models"
)

//go:embed fonts/DMSerifDisplay-Regular.ttf
var serifData []byte

//go:embed default.png
var defaultPNG []byte

const Width, Height = 1200, 630

var (
	cream = color.RGBA{248, 245, 236, 255}
	green = color.RGBA{46, 67, 55, 255}
	gold  = color.RGBA{191, 153, 75, 255}
)

func Render(wedding models.Wedding) ([]byte, error) {
	if strings.TrimSpace(wedding.PartnerOne) == "" || strings.TrimSpace(wedding.PartnerTwo) == "" || wedding.Date == nil {
		return nil, errors.New("wedding preview details are incomplete")
	}
	result, err := render(wedding, true)
	if err == nil && len(result) <= 300*1024 {
		return result, nil
	}
	return render(wedding, false)
}

func render(wedding models.Wedding, withPhoto bool) ([]byte, error) {
	canvas := image.NewRGBA(image.Rect(0, 0, Width, Height))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{cream}, image.Point{}, draw.Src)
	if photo := weddingPhoto(wedding.HeroImage); withPhoto && photo != nil {
		soft := image.NewRGBA(image.Rect(0, 0, Width/5, Height/5))
		xdraw.CatmullRom.Scale(soft, soft.Bounds(), photo, photo.Bounds(), xdraw.Over, nil)
		xdraw.CatmullRom.Scale(canvas, canvas.Bounds(), soft, soft.Bounds(), xdraw.Over, nil)
		draw.Draw(canvas, canvas.Bounds(), &image.Uniform{color.RGBA{248, 245, 236, 218}}, image.Point{}, draw.Over)
	}
	drawBorder(canvas)
	face, err := fontFace(68)
	if err != nil {
		return nil, err
	}
	faceSize := 68.0
	for font.MeasureString(face, wedding.PartnerOne+" &").Ceil() > 1040 || font.MeasureString(face, wedding.PartnerTwo).Ceil() > 1040 {
		faceSize -= 2
		face, err = fontFace(faceSize)
		if err != nil {
			return nil, err
		}
		if faceSize < 28 {
			break
		}
	}
	brand, err := fontFace(22)
	if err != nil {
		return nil, err
	}
	body, err := fontFace(25)
	if err != nil {
		return nil, err
	}
	centerText(canvas, brand, "W  WEDDINGHUB", 600, 122, gold)
	centerText(canvas, face, wedding.PartnerOne+" &", 600, 282, green)
	centerText(canvas, face, wedding.PartnerTwo, 600, 365, green)
	date := wedding.Date.Format("January 2, 2006")
	place := strings.TrimSpace(wedding.City)
	if place == "" {
		place = strings.TrimSpace(wedding.Venue)
	}
	if place != "" {
		date += "  ·  " + place
	}
	centerText(canvas, body, date, 600, 438, green)
	centerText(canvas, body, "YOU'RE INVITED", 600, 505, gold)
	var out bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.BestCompression}
	if err := encoder.Encode(&out, canvas); err != nil {
		return nil, err
	}
	if out.Len() > 300*1024 {
		return nil, errors.New("generated banner exceeds 300 KB")
	}
	return out.Bytes(), nil
}

func DefaultBanner() []byte {
	return append([]byte(nil), defaultPNG...)
}

func fontFace(size float64) (font.Face, error) {
	f, err := opentype.Parse(serifData)
	if err != nil {
		return nil, err
	}
	return opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
}

func centerText(dst *image.RGBA, face font.Face, value string, x, baseline int, ink color.Color) {
	width := font.MeasureString(face, value).Ceil()
	d := &font.Drawer{Dst: dst, Src: image.NewUniform(ink), Face: face, Dot: fixed.P(x-width/2, baseline)}
	d.DrawString(value)
}

func drawBorder(dst *image.RGBA) {
	draw.Draw(dst, image.Rect(24, 24, Width-24, 27), &image.Uniform{gold}, image.Point{}, draw.Src)
	draw.Draw(dst, image.Rect(24, Height-27, Width-24, Height-24), &image.Uniform{gold}, image.Point{}, draw.Src)
	draw.Draw(dst, image.Rect(24, 24, 27, Height-24), &image.Uniform{gold}, image.Point{}, draw.Src)
	draw.Draw(dst, image.Rect(Width-27, 24, Width-24, Height-24), &image.Uniform{gold}, image.Point{}, draw.Src)
}

func weddingPhoto(value string) image.Image {
	if strings.HasPrefix(value, "data:image/jpeg;base64,") {
		data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, "data:image/jpeg;base64,"))
		if err != nil || len(data) > 500_000 {
			return nil
		}
		return decodePhoto(data)
	}
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return nil
	}
	if parsed.Port() != "" && parsed.Port() != "443" {
		return nil
	}
	dialer := &net.Dialer{Timeout: 2 * time.Second}
	transport := &http.Transport{DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil || port != "443" {
			return nil, errors.New("unsafe photo endpoint")
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil || len(ips) == 0 {
			return nil, errors.New("photo host lookup failed")
		}
		for _, item := range ips {
			if !publicIP(item.IP) {
				return nil, errors.New("photo host is not public")
			}
		}
		return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
	}}
	client := http.Client{Timeout: 3 * time.Second, Transport: transport, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return errors.New("redirects are not allowed") }}
	response, err := client.Get(parsed.String())
	if err != nil {
		return nil
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil
	}
	if response.ContentLength > 8<<20 {
		return nil
	}
	limited, err := io.ReadAll(io.LimitReader(response.Body, (8<<20)+1))
	if err != nil || len(limited) > 8<<20 {
		return nil
	}
	return decodePhoto(limited)
}

func decodePhoto(data []byte) image.Image {
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width < 1 || config.Height < 1 || int64(config.Width)*int64(config.Height) > 24_000_000 {
		return nil
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	return img
}

func publicIP(ip net.IP) bool {
	return ip != nil && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !ip.IsLinkLocalMulticast() && !ip.IsUnspecified()
}
