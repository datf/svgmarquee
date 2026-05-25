# SVG Marquee Generator

A fast, dynamic Go application that generates seamlessly looping, animated SVG marquees for GitHub READMEs. 

**Why use this?**
GitHub imposes strict security limits on repository READMEs. It aggressively strips custom CSS animations and JavaScript from SVGs, and its image proxy (Camo) frequently freezes or breaks externally hosted animated GIFs. This tool bypasses those restrictions by using native SVG SMIL animations (`<animateTransform>`) and inline base64-encoded fonts. This guarantees a smooth, lightweight animation that GitHub's sanitizers won't block.

## Quick Start

You can generate an animated marquee immediately using the hosted instance at `https://marquee.datf.net`. Simply append your desired parameters to the URL and embed it in any Markdown file as an image.

**Markdown Example:**
```md
![Marquee](https://marquee.datf.net/?content=YOUR+•+TEXT+•+GOES+•+HERE+•+HELLO+WORLD!+•+&bg=FF5733&color=FFF&weight=400)
```
**Result:**
![Marquee](https://marquee.datf.net/?content=YOUR+•+TEXT+•+GOES+•+HERE+•+HELLO+WORLD!+•+&bg=FF5733&color=FFF&weight=400)

## URL Parameters

Customize your generated SVG by passing these query parameters to the URL.

| Parameter | Description | Default Value | Example |
|---|---|---|---|
| `content` | The text displayed in the marquee. Add spaces or symbols like bullets (•) at the end to space out the loop. | `YOUR • TEXT • GOES • HERE • ` | `?content=AWESOME+PROJECT` |
| `font` | The Google Fonts family name to use. The tool automatically fetches and embeds this font. | `Syne` | `?font=Inter` |
| `weight` | The font weight. | `600` | `?weight=800` |
| `size` | The font size in pixels. | `48` | `?size=64` |
| `color` / `pcolor` | The text color. Accepts hex codes (with or without `#`), named colors, or `transparent`. | `#FFFFFF` | `?color=ff0000` |
| `bg` / `background` | The background color of the marquee ribbon. | `#000000` | `?bg=transparent` |
| `duration` | Animation speed. Accepts seconds (`s`), milliseconds (`ms`), or raw numbers. | `20s` | `?duration=15s` |
| `width` | The total width of the SVG canvas. | `1000` | `?width=1200` |
| `height` | The total height of the SVG canvas. | `200` | `?height=300` |
| `rotate` | The rotation angle of the marquee ribbon in degrees. | `-3.0` | `?rotate=0` |
| `width_factor` | Multiplier used to calculate text width for seamless looping. Adjust if your custom font overlaps or leaves large gaps. | `0.52` | `?width_factor=0.6` |

## How It Works

1. **Font Embedding:** When a request is made, the application queries the Google Fonts API, subsets the font to only include the characters used in your `content`, downloads the raw `.woff2` font file, and encodes it into a base64 Data URI directly inside the SVG. This bypasses GitHub's external asset blocking.
2. **Caching:** To ensure fast response times and avoid hitting rate limits, generated CSS and encoded fonts are cached in memory for 24 hours.
3. **Animation:** The scrolling effect is achieved using native `<animateTransform>` tags within the SVG, calculating the exact translation width based on the `width_factor` and character count to create a perfect, infinite loop.

## Running the Server Locally

By default, the application is designed to run behind a web server (like Nginx) using FastCGI. If you want to test the application locally during development, you can start a standard HTTP server on port `8080` using the provided command-line flags.

```bash
# Run locally on http://localhost:8080
go run main.go -local

# Alternatively, use the dev alias
go run main.go -dev
```
