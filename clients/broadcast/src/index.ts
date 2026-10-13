// @couchverse/broadcast: Couch TV's cutscenes. The web imports this module (the admin preview);
// the render sidecar loads dist/render.html, whose API is in render.ts.

export { Player, type PlayerSnapshot, type PlayerState } from './player';
export { Stage, type Layer } from './stage';
export { TEMPLATES, TEMPLATE_NAMES } from './templates';
export { IDENT_DESIGNS, type IdentDesign } from './brand/idents';
export { PLACEHOLDER_BEDS } from './audio/placeholder';
export { airing, DAY_NAMES, type AiringKind } from './airing';
export { palette, pickAccent, type Palette } from './core/palette';
export { FPS, type Cue, type KeyRect } from './core/timeline';
export type * from './spec';
// the brand for the clients' live overlays (D14): the static lockup and its display face
export { CHANNEL, DISPLAY_FAMILY, lockupSvg, type LockupOptions } from './brand/wordmark';
export { ensureFonts } from './assets';
