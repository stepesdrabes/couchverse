#!/usr/bin/env bash
# Regenerates the stand-in media the fixtures and golden frames use: gradient artwork for four
# fictional titles, title logos, two short footage shots and a rhythmic bed. The files are committed
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

# two stand-in title logos, trimmed and transparent round their letters: a wide two-tone gold
# one (4:1) and a squarer red and paper one (2.3:1); each box is colour:x:y:w:h
logo() { # file size box...
	local file=$1 size=$2 chain=""
	shift 2
	for b in "$@"; do
		IFS=: read -r c x y w h <<<"$b"
		chain+="${chain:+,}drawbox=x=$x:y=$y:w=$w:h=$h:color=$c:t=fill:replace=1"
	done
	ff -f lavfi -i "color=c=black@0:s=$size,format=rgba" -frames:v 1 -vf "$chain" "$file"
}
G=0xe2b866
B=0x9a6a24
logo logo-wide.png 800x200 \
	$G:0:0:150:40 $G:55:40:40:160 $G:190:0:40:200 $G:300:0:40:200 $G:230:80:70:40 \
	$G:380:0:40:200 $G:420:0:110:40 $G:420:80:80:40 $G:420:160:110:40 $G:570:0:40:200 \
	$G:690:0:40:200 $G:610:40:26:50 $G:636:90:28:50 $G:664:140:26:40 $G:750:0:50:40 \
	$G:750:160:50:40 $B:55:160:40:40 $B:190:160:40:40 $B:300:160:40:40 $B:380:160:40:40 \
	$B:420:160:110:40 $B:570:160:40:40 $B:690:160:40:40 $B:664:160:26:20 $B:750:160:50:40
logo logo-tall.png 460x200 \
	0xd81d24:0:0:200:200 black@0:50:50:100:100 0xf2ece2:240:0:220:60 0xf2ece2:240:90:160:40 \
	0xd81d24:240:160:220:40

# the strip and short promos' logos, trimmed PNGs on transparency: blocks in yellow outlined in
# near-black (they read on night by their fill and on paper by their outline) and a flat dark red
# (too dark for night alone)
blocklogo() { # name size boxes...
	local name=$1 size=$2 vf="format=rgba"
	shift 2
	for b in "$@"; do vf="$vf,drawbox=$b:t=fill:replace=1"; done
	ff -f lavfi -i "color=c=0x00000000:s=$size,format=rgba" -frames:v 1 -vf "$vf" "logo-$name.png"
}
blocklogo sunny 1200x400 \
	x=0:y=0:w=360:h=400:color=0x141414@1 x=420:y=60:w=360:h=340:color=0x141414@1 \
	x=840:y=0:w=360:h=400:color=0x141414@1 x=18:y=18:w=324:h=364:color=0xffd51c@1 \
	x=438:y=78:w=324:h=304:color=0xffd51c@1 x=858:y=18:w=324:h=364:color=0xffd51c@1 \
	x=120:y=120:w=120:h=160:color=0x141414@1 x=960:y=200:w=120:h=182:color=0x00000000@0
blocklogo water 1000x420 \
	x=0:y=0:w=260:h=420:color=0x8a1c22@1 x=330:y=100:w=260:h=320:color=0x8a1c22@1 \
	x=660:y=0:w=340:h=420:color=0x8a1c22@1 x=740:y=120:w=180:h=180:color=0x00000000@0
