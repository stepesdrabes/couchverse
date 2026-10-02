/** letters in a pairing code; a device shows it as XXXX-XXXX */
export const PAIRING_CODE_LENGTH = 8;

/** what a person types or pastes ("wdjb mjht", "WDJB-MJHT") down to the code's letters */
export const pairingLetters = (raw: string): string =>
	raw
		.toUpperCase()
		.replace(/[^A-Z]/g, '')
		.slice(0, PAIRING_CODE_LENGTH);

/** "wdjbmjht" -> "WDJB-MJHT", the way the device shows it */
export function formatPairingCode(raw: string): string {
	const code = pairingLetters(raw);
	return code.length > 4 ? `${code.slice(0, 4)}-${code.slice(4)}` : code;
}

export const isCompletePairingCode = (raw: string): boolean =>
	pairingLetters(raw).length === PAIRING_CODE_LENGTH;
