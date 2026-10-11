import { afterAll } from 'vitest';

// @wailsio/runtime starts a 50 ms timer when it loads, and the timer reads
// `window`. A test file that finishes sooner tears jsdom down first, and the
// timer then fails the run with "window is not defined". Let it fire first.
afterAll(() => new Promise((resolve) => setTimeout(resolve, 100)));
