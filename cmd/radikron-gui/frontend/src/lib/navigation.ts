// The screens reachable from the sidebar, in sidebar order.
export const screens = [
  { id: 'overview', label: 'Overview' },
  { id: 'downloads', label: 'Downloads' },
  { id: 'timetable', label: 'Timetable' },
  { id: 'search', label: 'Search' },
  { id: 'rules', label: 'Rules' },
  { id: 'settings', label: 'Settings' },
] as const;

export type ScreenId = (typeof screens)[number]['id'];
