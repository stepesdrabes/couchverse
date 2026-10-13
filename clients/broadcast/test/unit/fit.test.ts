import { describe, expect, it } from 'vitest';
import { breakLines, type Word } from '../../src/core/fit';

// a monospace stand-in for the measured words: 10 px a letter, 5 px a space
function measured(text: string): Word[] {
	let x = 0;
	return text.split(' ').map((w) => {
		const word = { text: w, left: x, right: x + w.length * 10 };
		x = word.right + 5;
		return word;
	});
}

function set(text: string, lines: number, max?: number) {
	const words = measured(text);
	const starts = breakLines(words, lines, max);
	if (!starts) return null;
	const ends = [...starts, words.length];
	return ends.map((e, i) =>
		words
			.slice(i ? ends[i - 1] : 0, e)
			.map((w) => w.text)
			.join(' ')
	);
}

describe('breakLines', () => {
	it('balances the lines', () => {
		expect(set('Harry Potter and the Deathly Hallows: Part 2', 2)).toEqual([
			'Harry Potter and the',
			'Deathly Hallows: Part 2'
		]);
		expect(set('Harry Potter and the Philosopher’s Stone', 2)).toEqual([
			'Harry Potter and the',
			'Philosopher’s Stone'
		]);
		expect(set('Harry Potter and the Deathly Hallows: Part 2', 3)).toEqual([
			'Harry Potter',
			'and the Deathly',
			'Hallows: Part 2'
		]);
	});

	it('breaks after a colon or before "the" only when that stays balanced', () => {
		expect(set('The Lord of the Rings: The Return of the King', 2)).toEqual([
			'The Lord of the Rings:',
			'The Return of the King'
		]);
		expect(set('Star Wars: The Empire Strikes Back', 2)).toEqual([
			'Star Wars:',
			'The Empire Strikes Back'
		]);
		expect(set('Dr. Strangelove or: How I Learned to Stop Worrying and Love the Bomb', 3)).toEqual([
			'Dr. Strangelove or:',
			'How I Learned to Stop',
			'Worrying and Love the Bomb'
		]);
		// "...Hallows: / Part 2" would leave the second line a stub
		expect(set('Harry Potter and the Deathly Hallows: Part 2', 2)?.[1]).not.toBe('Part 2');
	});

	it('never leaves one word alone on the last line', () => {
		expect(set('ŽABKY NA TĚLÁK', 2)).toEqual(['ŽABKY', 'NA TĚLÁK']);
		expect(set('Something wicked this way comes.', 3)?.at(-1)?.split(' ').length).toBe(2);
		// with as many lines as words every line has one
		expect(set('Věřte nevěřte', 2)).toEqual(['Věřte', 'nevěřte']);
	});

	it('keeps a one-letter word off the end of a line', () => {
		expect(set('Pat a Mat a nová dobrodružství', 2)).toEqual(['Pat a Mat', 'a nová dobrodružství']);
	});

	it('never starts a line with a dash', () => {
		const lines = set('Mission: Impossible \u2013 Dead Reckoning Part One', 3) ?? [];
		expect(lines).toHaveLength(3);
		for (const l of lines) expect(l.startsWith('\u2013')).toBe(false);
	});

	it('keeps every line within the widest allowed', () => {
		expect(set('Harry Potter and the Deathly Hallows: Part 2', 2, 200)).toBeNull();
		expect(set('Harry Potter and the Deathly Hallows: Part 2', 3, 140)).toEqual([
			'Harry Potter',
			'and the Deathly',
			'Hallows: Part 2'
		]);
		expect(set('Věřte nevěřte', 1, 1000)).toEqual(['Věřte nevěřte']);
		expect(set('Věřte nevěřte', 1, 100)).toBeNull();
		expect(set('Věřte', 2)).toBeNull();
	});
});
