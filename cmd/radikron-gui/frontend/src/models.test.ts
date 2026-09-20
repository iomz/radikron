import { describe, it, expect } from 'vitest';
import { radikron } from '../wailsjs/go/models';

describe('radikron.Rule', () => {
  it('keeps the exclude block when a rule is rebuilt during editing', () => {
    // RulesEditor rebuilds rules with Rule.createFrom on every field edit.
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

  it('leaves Exclude undefined when a rule has none', () => {
    const rule = radikron.Rule.createFrom({ Name: 'plain', Title: 'X' });
    expect(rule.Exclude).toBeUndefined();
  });
});
