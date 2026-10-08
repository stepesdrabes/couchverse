"""Decide how to transcode a source: smart-copy vs NVENC, which audio stream,
and language normalization. Pure logic over a ProbeResult - no ffmpeg here."""

from __future__ import annotations

from dataclasses import dataclass, field
import re

from .ffprobe import AudioStream, ProbeResult

# Codecs the server can turn into WebVTT (bitmap subs need OCR - skipped).
TEXT_SUBTITLE_CODECS = {"subrip", "srt", "ass", "ssa", "mov_text", "webvtt"}

# ISO 639-2 (bibliographic and terminological) -> 639-1, for the common languages.
# ffprobe reports 3-letter container tags; Couchverse wants 2-letter codes.
_ISO3_TO_ISO1 = {
    "eng": "en", "cze": "cs", "ces": "cs", "slo": "sk", "slk": "sk",
    "ger": "de", "deu": "de", "fre": "fr", "fra": "fr", "spa": "es",
    "ita": "it", "pol": "pl", "rus": "ru", "jpn": "ja", "kor": "ko",
    "chi": "zh", "zho": "zh", "por": "pt", "dut": "nl", "nld": "nl",
    "hun": "hu", "rum": "ro", "ron": "ro", "ukr": "uk", "tur": "tr",
    "ara": "ar", "heb": "he", "gre": "el", "ell": "el", "dan": "da",
    "swe": "sv", "nor": "no", "fin": "fi", "tha": "th", "vie": "vi",
    "hin": "hi", "ind": "id", "bul": "bg", "hrv": "hr", "srp": "sr",
    "slv": "sl", "lit": "lt", "lav": "lv", "est": "et", "cat": "ca",
}

# Terminological ISO 639-2 tags are required for MP4's three-letter language field.
_ISO1_TO_ISO3 = dict(pair.split("=") for pair in """
aa=aar ab=abk ae=ave af=afr ak=aka am=amh an=arg ar=ara as=asm av=ava ay=aym az=aze
ba=bak be=bel bg=bul bi=bis bm=bam bn=ben bo=bod br=bre bs=bos ca=cat ce=che ch=cha
co=cos cr=cre cs=ces cu=chu cv=chv cy=cym da=dan de=deu dv=div dz=dzo ee=ewe el=ell
en=eng eo=epo es=spa et=est eu=eus fa=fas ff=ful fi=fin fj=fij fo=fao fr=fra fy=fry
ga=gle gd=gla gl=glg gn=grn gu=guj gv=glv ha=hau he=heb hi=hin ho=hmo hr=hrv ht=hat
hu=hun hy=hye hz=her ia=ina id=ind ie=ile ig=ibo ii=iii ik=ipk io=ido is=isl it=ita
iu=iku ja=jpn jv=jav ka=kat kg=kon ki=kik kj=kua kk=kaz kl=kal km=khm kn=kan ko=kor
kr=kau ks=kas ku=kur kv=kom kw=cor ky=kir la=lat lb=ltz lg=lug li=lim ln=lin lo=lao
lt=lit lu=lub lv=lav mg=mlg mh=mah mi=mri mk=mkd ml=mal mn=mon mr=mar ms=msa mt=mlt
my=mya na=nau nb=nob nd=nde ne=nep ng=ndo nl=nld nn=nno no=nor nr=nbl nv=nav ny=nya
oc=oci oj=oji om=orm or=ori os=oss pa=pan pi=pli pl=pol ps=pus pt=por qu=que rm=roh
rn=run ro=ron ru=rus rw=kin sa=san sc=srd sd=snd se=sme sg=sag si=sin sk=slk sl=slv
sm=smo sn=sna so=som sq=sqi sr=srp ss=ssw st=sot su=sun sv=swe sw=swa ta=tam te=tel
tg=tgk th=tha ti=tir tk=tuk tl=tgl tn=tsn to=ton tr=tur ts=tso tt=tat tw=twi ty=tah
ug=uig uk=ukr ur=urd uz=uzb ve=ven vi=vie vo=vol wa=wln wo=wol xh=xho yi=yid yo=yor
za=zha zh=zho zu=zul
""".split())
_ISO3_TO_ISO1.update({iso3: iso1 for iso1, iso3 in _ISO1_TO_ISO3.items()})
_ISO3_TO_ISO1.update(dict(zip(
    "alb arm baq bur geo ice mac mao may per tib wel".split(),
    "sq hy eu my ka is mk mi ms fa bo cy".split(),
)))


def normalize_lang(code: str) -> str:
    """Best-effort 2-letter ISO 639-1. Passes through unknown/2-letter codes."""
    c = (code or "").strip().lower().replace("_", "-").split("-", 1)[0]
    if not c or c in ("und", "unknown"):
        return ""
    if len(c) == 2:
        return "cs" if c == "cz" else c
    return _ISO3_TO_ISO1.get(c, c)


def parse_language_preferences(value: str) -> list[str]:
    """Parse language codes in preference order; an empty list selects the first track."""
    result: list[str] = []
    for token in re.split(r"[,;\s]+", (value or "").strip()):
        if not token:
            continue
        language = normalize_lang(token)
        if language not in _ISO1_TO_ISO3:
            raise ValueError(f"Invalid audio language '{token}'. Use codes such as en, cs or de.")
        if language not in result:
            result.append(language)
    return result


def preferred_language(value: str) -> str:
    preferences = parse_language_preferences(value)
    return preferences[0] if preferences else ""


class PlanError(ValueError):
    pass


@dataclass
class PlannedAudio:
    stream: AudioStream
    map: str
    language: str
    copy: bool
    downmix: bool


@dataclass
class TranscodePlan:
    video_copy: bool
    has_audio: bool
    audio_copy: bool
    audio_map: str            # e.g. "0:a:0"; "" when no audio
    audio_downmix: bool       # add -ac 2
    audio_lang_matched: bool  # False -> we fell back to the first stream
    chosen_audio: AudioStream | None
    video_map: str = "0:v:0"
    audio_language: str = ""
    hdr_tonemap: bool = False
    color_transfer: str = ""
    color_primaries: str = ""
    color_space: str = ""
    color_range: str = ""
    video_resize: bool = False
    output_width: int = 0
    output_height: int = 0
    audio_tracks: list[PlannedAudio] = field(default_factory=list)
    missing_audio_languages: list[str] = field(default_factory=list)
    audio_mode: str = "single"

    @property
    def video_action(self) -> str:
        return "remux" if self.video_copy else "encode"

    @property
    def multi_audio(self) -> bool:
        return len(self.audio_tracks) > 1

    @property
    def audio_languages(self) -> list[str]:
        return [track.language for track in self.audio_tracks if track.language]


def mp4_language_tag(language: str) -> str:
    return _ISO1_TO_ISO3.get(normalize_lang(language), "und")


def _audio_language(stream: AudioStream) -> str:
    language = normalize_lang(stream.language)
    return language if language in _ISO1_TO_ISO3 else ""


def _planned_audio(stream: AudioStream, keep_channels: bool) -> PlannedAudio:
    downmix = not keep_channels and stream.channels > 2
    return PlannedAudio(stream, f"0:a:{stream.typed_index}", _audio_language(stream),
                        stream.codec == "aac" and not downmix, downmix)


def pick_audio_stream(probe: ProbeResult, want_lang: str) -> tuple[AudioStream | None, bool]:
    """Choose the audio stream matching want_lang; fall back to the first. Returns
    (stream, matched)."""
    if not probe.audio:
        return None, False
    for want in parse_language_preferences(want_lang):
        for a in probe.audio:
            if _audio_language(a) == want:
                return a, True
    return probe.audio[0], False


def build_plan(probe: ProbeResult, want_lang: str, keep_channels: bool,
               max_height: int = 0, audio_mode: str = "single") -> TranscodePlan:
    """Smart-copy safe streams, or encode H.264 with selected AAC audio tracks.

    Video is copied only when it is SDR h264 with 8-bit 4:2:0 chroma. Audio is
    copied only when it is already AAC and already within the target channel
    layout; otherwise it is re-encoded to AAC (stereo unless keep_channels)."""
    if max_height not in {0, 480, 720, 1080, 2160}:
        raise PlanError("Choose Original, 480p, 720p, 1080p or 2160p output resolution.")
    if audio_mode not in {"single", "multiple"}:
        raise PlanError("Choose single or multiple audio language mode.")
    preferences = parse_language_preferences(want_lang)
    if audio_mode == "multiple" and not preferences:
        raise PlanError("Multiple audio mode requires at least one language code, such as en, cs.")
    if not probe.has_video:
        raise PlanError("No playable video stream was found in this file.")
    output_width, output_height = target_dimensions(probe.width, probe.height, max_height)
    video_resize = (output_width, output_height) != (probe.width, probe.height)
    if probe.dovi_profile == 5:
        raise PlanError("Dolby Vision profile 5 has no HDR10-compatible base. Use an HDR10 or SDR source.")

    transfer = probe.color_transfer
    if probe.dovi_profile:
        if probe.dovi_profile == 7 or probe.dovi_compatibility_id == 1:
            transfer = "smpte2084"
        elif probe.dovi_compatibility_id == 4:
            transfer = "arib-std-b67"
        elif probe.dovi_profile == 9 or probe.dovi_compatibility_id == 2:
            transfer = "bt709"
        else:
            raise PlanError("This Dolby Vision source has no supported HDR10, HLG or SDR base layer.")
    hdr_tonemap = transfer in {"smpte2084", "arib-std-b67"}
    video_copy = (probe.video_codec == "h264"
                  and probe.pix_fmt.lower() in {"yuv420p", "yuvj420p"}
                  and not probe.is_10bit and not probe.is_hdr and not video_resize)
    if not video_copy:
        output_width += output_width % 2
        output_height += output_height % 2

    missing: list[str] = []
    tracks: list[PlannedAudio] = []
    if audio_mode == "multiple":
        for language in preferences:
            stream = next((a for a in probe.audio if _audio_language(a) == language), None)
            if stream is None:
                missing.append(language)
            else:
                tracks.append(_planned_audio(stream, keep_channels))
        if not tracks:
            raise PlanError(f"No requested audio languages were found: {', '.join(preferences)}.")
        matched = True
    else:
        chosen, matched = pick_audio_stream(probe, want_lang)
        if chosen is not None:
            tracks.append(_planned_audio(chosen, keep_channels))
    first = tracks[0] if tracks else None
    return TranscodePlan(
        video_copy=video_copy,
        has_audio=bool(tracks),
        audio_copy=first.copy if first else False,
        audio_map=first.map if first else "",
        audio_downmix=first.downmix if first else False,
        audio_lang_matched=matched,
        chosen_audio=first.stream if first else None,
        video_map=f"0:{probe.video_index}",
        audio_language=first.language if first else "",
        hdr_tonemap=hdr_tonemap,
        color_transfer=transfer,
        color_primaries=probe.color_primaries,
        color_space=probe.color_space,
        color_range=probe.color_range,
        video_resize=video_resize,
        output_width=output_width,
        output_height=output_height,
        audio_tracks=tracks,
        missing_audio_languages=missing,
        audio_mode=audio_mode,
    )


def target_dimensions(width: int, height: int, max_height: int) -> tuple[int, int]:
    """Fit inside the selected 16:9 box, preserving aspect and avoiding enlargement."""
    if not max_height:
        return width, height
    if width <= 0 or height <= 0:
        raise PlanError("Could not determine the source dimensions for resizing.")
    max_width = {480: 854, 720: 1280, 1080: 1920, 2160: 3840}.get(max_height)
    if max_width is None:
        raise PlanError("Unsupported output resolution.")
    if width <= max_width and height <= max_height:
        return width, height
    ratio = min(max_width / width, max_height / height)
    return max(2, int(width * ratio) // 2 * 2), max(2, int(height * ratio) // 2 * 2)
