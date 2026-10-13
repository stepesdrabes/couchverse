#!/usr/bin/env bash
# Regenerates the stand-in media the fixtures and golden frames use: gradient artwork for four
# fictional titles, a logo, two short footage shots and a rhythmic bed. The files are committed
# (golden frames depend on their exact bytes); rerun only to change them, then update the goldens.
set -euo pipefail
cd "$(dirname "$0")"
ff() { ffmpeg -hide_banner -loglevel error -y "$@"; }

art() { # name c0 c1 c2
	ff -f lavfi -i "gradients=s=1280x720:c0=$2:c1=$3:x0=0:y0=0:x1=1280:y1=720:nb_colors=2:d=1" -frames:v 1 \
		-vf "drawbox=x=760:y=120:w=300:h=480:color=$4@0.35:t=fill,drawbox=x=120:y=470:w=520:h=140:color=white@0.12:t=fill,drawbox=x=0:y=640:w=1280:h=80:color=black@0.35:t=fill" \
		-q:v 4 "$1-backdrop.jpg"
	ff -f lavfi -i "gradients=s=500x750:c0=$3:c1=$2:x0=0:y0=0:x1=500:y1=750:nb_colors=2:d=1" -frames:v 1 \
		-vf "drawbox=x=60:y=80:w=380:h=380:color=$4@0.45:t=fill,drawbox=x=60:y=560:w=380:h=60:color=white@0.5:t=fill" \
		-q:v 4 "$1-poster.jpg"
}
art night 0x0a2a44 0x3a6ea5 0x9fd3ff
art alley 0x643906 0xd62316 0xffd28a
art sunny 0xf5cc36 0xfed51c 0x2a62ff
art water 0xd69a77 0xd81d24 0x101828

# a title's logo: a PNG trimmed to its letters (four slanted gold blocks), transparent around them
slant="X-0.3*(240-Y)"
ff -f lavfi -i "color=c=black:s=892x240,format=rgba" -frames:v 1 \
	-vf "geq=r='236-0.25*Y':g='184-0.45*Y':b='80-0.2*Y':a='255*gt(between($slant,0,150)+between($slant,190,400)*not(between(Y,70,170)*between($slant,250,340))+between($slant,440,560)+between($slant,600,820)*not(between(Y,0,80)*between($slant,660,760)),0)'" \
	-pix_fmt rgba night-logo.png

mkdir -p shot-a shot-b
ff -f lavfi -i "testsrc2=s=480x270:r=25:d=1" -q:v 6 shot-a/%03d.jpg
ff -f lavfi -i "gradients=s=480x270:r=25:d=1:c0=0x3a6ea5:c1=0x0a2a44:speed=0.08" -q:v 6 shot-b/%03d.jpg

# kicks on the beat at 120 bpm, a tick off the beat, a deeper accent every two seconds
ff -f lavfi -i "aevalsrc='0.7*sin(2*PI*55*t)*exp(-10*mod(t,0.5))+0.12*sin(2*PI*3520*t)*exp(-40*mod(t+0.25,0.5))+0.45*sin(2*PI*110*t)*exp(-4*mod(t,2))':s=48000:d=24" \
	-ac 2 -c:a libopus -b:a 48k bed.ogg
