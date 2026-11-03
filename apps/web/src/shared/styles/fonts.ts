import { Anton, Instrument_Serif, Manrope, Space_Mono } from 'next/font/google';

// Self-hosted at build time by next/font: no request to Google at runtime, no layout shift.
const anton = Anton({ weight: '400', subsets: ['latin'], variable: '--font-anton', display: 'swap' });
const instrument = Instrument_Serif({ weight: '400', style: ['normal', 'italic'], subsets: ['latin'], variable: '--font-instrument', display: 'swap' });
const manrope = Manrope({ subsets: ['latin'], variable: '--font-manrope', display: 'swap' });
const spaceMono = Space_Mono({ weight: ['400', '700'], subsets: ['latin'], variable: '--font-space-mono', display: 'swap' });

/** Class names that define the font CSS variables; put on <html>. */
export const fontVariables = [anton.variable, instrument.variable, manrope.variable, spaceMono.variable].join(' ');
