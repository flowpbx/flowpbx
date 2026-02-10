/** Flow node type definitions — maps node type strings to metadata for the palette and canvas. */

export interface NodeTypeInfo {
  type: string
  label: string
  /** Short description shown in palette tooltip. */
  description: string
  /** Entity type this node references (if any). */
  entityType?: string
  /** Number of output handles (0 = terminal). -1 = dynamic (depends on config). */
  outputs: number | 'dynamic'
  /** Named output handles for static outputs. */
  outputHandles?: { id: string; label: string }[]
  /** SVG path for a compact icon (20x20 viewBox). */
  iconPath: string
  /** Tailwind bg/border color class suffix (e.g., 'blue' for bg-blue-50). */
  color: string
}

export const NODE_TYPES: NodeTypeInfo[] = [
  {
    type: 'inbound_number',
    label: 'Inbound Number',
    description: 'Entry point — one or more DIDs mapped to it',
    entityType: 'inbound_number',
    outputs: 1,
    outputHandles: [{ id: 'next', label: 'Next' }],
    iconPath: 'M2 3a1 1 0 011-1h2.153a1 1 0 01.986.836l.74 4.435a1 1 0 01-.54 1.06l-1.548.773a11.037 11.037 0 006.105 6.105l.774-1.548a1 1 0 011.059-.54l4.435.74a1 1 0 01.836.986V17a1 1 0 01-1 1h-2C7.82 18 2 12.18 2 5V3z',
    color: 'emerald',
  },
  {
    type: 'time_switch',
    label: 'Time Switch',
    description: 'Route calls based on day/time rules',
    entityType: 'time_switch',
    outputs: 'dynamic',
    iconPath: 'M10 18a8 8 0 100-16 8 8 0 000 16zm1-12a1 1 0 10-2 0v4a1 1 0 00.293.707l2.828 2.829a1 1 0 101.415-1.415L11 9.586V6z',
    color: 'amber',
  },
  {
    type: 'ivr_menu',
    label: 'IVR Menu',
    description: 'Play prompt, collect DTMF digits',
    entityType: 'ivr_menu',
    outputs: 'dynamic',
    iconPath: 'M3 5a1 1 0 011-1h12a1 1 0 110 2H4a1 1 0 01-1-1zm0 5a1 1 0 011-1h12a1 1 0 110 2H4a1 1 0 01-1-1zm0 5a1 1 0 011-1h6a1 1 0 110 2H4a1 1 0 01-1-1z',
    color: 'violet',
  },
  {
    type: 'ring_group',
    label: 'Ring Group',
    description: 'Ring a group of extensions',
    entityType: 'ring_group',
    outputs: 2,
    outputHandles: [
      { id: 'answered', label: 'Answered' },
      { id: 'no_answer', label: 'No Answer' },
    ],
    iconPath: 'M13 6a3 3 0 11-6 0 3 3 0 016 0zm5 2a2 2 0 11-4 0 2 2 0 014 0zm-4 7a4 4 0 00-8 0v3h8v-3zM6 8a2 2 0 11-4 0 2 2 0 014 0zm10 10v-3a5.972 5.972 0 00-.75-2.906A3.005 3.005 0 0119 15v3h-3zM4.75 12.094A5.973 5.973 0 004 15v3H1v-3a3 3 0 013.75-2.906z',
    color: 'blue',
  },
  {
    type: 'extension',
    label: 'Extension',
    description: 'Ring a single extension',
    entityType: 'extension',
    outputs: 2,
    outputHandles: [
      { id: 'answered', label: 'Answered' },
      { id: 'no_answer', label: 'No Answer' },
    ],
    iconPath: 'M10 9a3 3 0 100-6 3 3 0 000 6zm-7 9a7 7 0 1114 0H3z',
    color: 'sky',
  },
  {
    type: 'voicemail',
    label: 'Voicemail',
    description: 'Record voicemail into a mailbox',
    entityType: 'voicemail_box',
    outputs: 1,
    outputHandles: [{ id: 'after_recording', label: 'After Recording' }],
    iconPath: 'M18 10c0 3.866-3.582 7-8 7a8.841 8.841 0 01-4.083-.98L2 17l1.338-3.123C2.493 12.767 2 11.434 2 10c0-3.866 3.582-7 8-7s8 3.134 8 7z',
    color: 'orange',
  },
  {
    type: 'play_message',
    label: 'Play Message',
    description: 'Play an audio file or TTS',
    outputs: 1,
    outputHandles: [{ id: 'after_playback', label: 'After Playback' }],
    iconPath: 'M9.383 3.076A1 1 0 0110 4v12a1 1 0 01-1.707.707L4.586 13H2a1 1 0 01-1-1V8a1 1 0 011-1h2.586l3.707-3.707a1 1 0 011.09-.217zM14.657 2.929a1 1 0 011.414 0A9.972 9.972 0 0119 10a9.972 9.972 0 01-2.929 7.071 1 1 0 01-1.414-1.414A7.971 7.971 0 0017 10c0-2.21-.894-4.208-2.343-5.657a1 1 0 010-1.414z',
    color: 'teal',
  },
  {
    type: 'conference',
    label: 'Conference',
    description: 'Join caller into a conference bridge',
    entityType: 'conference',
    outputs: 1,
    outputHandles: [{ id: 'after_leave', label: 'After Leave' }],
    iconPath: 'M7 4a3 3 0 016 0v4a3 3 0 11-6 0V4zm4 10.93A7.001 7.001 0 0017 8a1 1 0 10-2 0A5 5 0 015 8a1 1 0 00-2 0 7.001 7.001 0 006 6.93V17H6a1 1 0 100 2h8a1 1 0 100-2h-3v-2.07z',
    color: 'indigo',
  },
  {
    type: 'transfer',
    label: 'Transfer',
    description: 'Transfer call to external number or extension',
    outputs: 0,
    iconPath: 'M10.293 3.293a1 1 0 011.414 0l6 6a1 1 0 010 1.414l-6 6a1 1 0 01-1.414-1.414L14.586 11H3a1 1 0 110-2h11.586l-4.293-4.293a1 1 0 010-1.414z',
    color: 'cyan',
  },
  {
    type: 'hangup',
    label: 'Hangup',
    description: 'Terminate the call',
    outputs: 0,
    iconPath: 'M3.707 2.293a1 1 0 00-1.414 1.414l6.921 6.922c.05.062.105.118.168.167l6.91 6.911a1 1 0 001.415-1.414l-.354-.354A8 8 0 003.707 2.293zM2 10a8 8 0 0011.953 6.953l-1.5-1.5A6 6 0 014 10a1 1 0 00-2 0zm14-1a1 1 0 10-2 0 4 4 0 01-.32 1.564l1.492 1.492A6 6 0 0016 10v-1z',
    color: 'red',
  },
  {
    type: 'set_caller_id',
    label: 'Set Caller ID',
    description: 'Override caller ID name/number',
    outputs: 1,
    outputHandles: [{ id: 'next', label: 'Next' }],
    iconPath: 'M17.414 2.586a2 2 0 00-2.828 0L7 10.172V13h2.828l7.586-7.586a2 2 0 000-2.828z M2 6a2 2 0 012-2h4a1 1 0 010 2H4v10h10v-4a1 1 0 112 0v4a2 2 0 01-2 2H4a2 2 0 01-2-2V6z',
    color: 'pink',
  },
]

/** Look up node type info by type string. */
export function getNodeTypeInfo(type: string): NodeTypeInfo | undefined {
  return NODE_TYPES.find((n) => n.type === type)
}

/** Color map — Tailwind classes for each node color. */
export const NODE_COLORS: Record<string, { bg: string; border: string; text: string; handle: string }> = {
  emerald: { bg: 'bg-emerald-50', border: 'border-emerald-300', text: 'text-emerald-700', handle: 'bg-emerald-500' },
  amber:   { bg: 'bg-amber-50',   border: 'border-amber-300',   text: 'text-amber-700',   handle: 'bg-amber-500' },
  violet:  { bg: 'bg-violet-50',  border: 'border-violet-300',  text: 'text-violet-700',  handle: 'bg-violet-500' },
  blue:    { bg: 'bg-blue-50',    border: 'border-blue-300',    text: 'text-blue-700',    handle: 'bg-blue-500' },
  sky:     { bg: 'bg-sky-50',     border: 'border-sky-300',     text: 'text-sky-700',     handle: 'bg-sky-500' },
  orange:  { bg: 'bg-orange-50',  border: 'border-orange-300',  text: 'text-orange-700',  handle: 'bg-orange-500' },
  teal:    { bg: 'bg-teal-50',    border: 'border-teal-300',    text: 'text-teal-700',    handle: 'bg-teal-500' },
  indigo:  { bg: 'bg-indigo-50',  border: 'border-indigo-300',  text: 'text-indigo-700',  handle: 'bg-indigo-500' },
  cyan:    { bg: 'bg-cyan-50',    border: 'border-cyan-300',    text: 'text-cyan-700',    handle: 'bg-cyan-500' },
  red:     { bg: 'bg-red-50',     border: 'border-red-300',     text: 'text-red-700',     handle: 'bg-red-500' },
  pink:    { bg: 'bg-pink-50',    border: 'border-pink-300',    text: 'text-pink-700',    handle: 'bg-pink-500' },
}
