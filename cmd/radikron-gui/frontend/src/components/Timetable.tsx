import React from 'react';

// Timetable will show the broadcast day with each scheduled program placed
// on a time axis. It is built in D2 of the GUI redesign (#77).
export const Timetable: React.FC = () => (
  <div className="flex flex-col gap-2">
    <h1 className="m-0 text-[28px] font-semibold">Timetable</h1>
    <p className="text-muted-foreground">The broadcast-day timetable arrives with the redesigned screens.</p>
  </div>
);
