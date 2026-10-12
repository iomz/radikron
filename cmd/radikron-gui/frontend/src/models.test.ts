import { describe, it, expect } from 'vitest';
import * as radikron from '../bindings/github.com/iomz/radikron';

describe('radikron.Rule', () => {
  it('keeps the exclude block when a rule is rebuilt during editing', () => {
    // The Rules screen rebuilds rules with Rule.createFrom on every field edit.
    // Fields the model does not declare are dropped, so a hand-written
    // exclude block would be silently lost when the user edits any field.
    const loaded = radikron.Rule.createFrom({
      Name: 'midday',
      Title: 'MIDDAY LOUNGE',
      StationID: 'FMJ',
      Exclude: { Pfm: 'GUEST', DoW: ['sat'] },
    });

    const edited = radikron.Rule.createFrom({ ...loaded, Folder: 'MIDDAY' });

    expect(edited.Exclude).toBeDefined();
    expect(edited.Exclude?.Pfm).toBe('GUEST');
    expect(edited.Exclude?.DoW).toEqual(['sat']);
    expect(edited.Folder).toBe('MIDDAY');
  });

  it('leaves Exclude empty when a rule has none', () => {
    const rule = radikron.Rule.createFrom({ Name: 'plain', Title: 'X' });
    // Wails v3 models represent an absent pointer as null; Go decodes it to nil.
    expect(rule.Exclude).toBeNull();
  });
});
