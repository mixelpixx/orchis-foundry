// Hand-crafted minimal line icons. 1.5px stroke, 16px nominal.
const Icon = ({ d, size = 16, stroke = 1.5, fill = "none", style, ...rest }) => (
  <svg width={size} height={size} viewBox="0 0 16 16" fill={fill} stroke="currentColor"
    strokeWidth={stroke} strokeLinecap="round" strokeLinejoin="round" style={style} {...rest}>
    {typeof d === "string" ? <path d={d} /> : d}
  </svg>
);

const Icons = {
  Home: (p) => <Icon {...p} d={<><path d="M2.5 7l5.5-4.5L13.5 7v6.5h-3v-4h-4v4h-3z"/></>} />,
  Repo: (p) => <Icon {...p} d={<><path d="M3 2.5h8.5a1.5 1.5 0 0 1 1.5 1.5v9.5"/><path d="M3 2.5v9a1 1 0 0 0 1 1h9"/><path d="M5.5 5.5h4"/></>} />,
  PR: (p) => <Icon {...p} d={<><circle cx="4" cy="4" r="1.5"/><circle cx="4" cy="12" r="1.5"/><circle cx="12" cy="12" r="1.5"/><path d="M4 5.5v5"/><path d="M10.5 12H8a3.5 3.5 0 0 1-3.5-3.5V5.5"/></>} />,
  Issue: (p) => <Icon {...p} d={<><circle cx="8" cy="8" r="5.5"/><path d="M8 5.5v3M8 10.4v.1"/></>} />,
  Search: (p) => <Icon {...p} d={<><circle cx="7" cy="7" r="4.5"/><path d="M10.5 10.5l3 3"/></>} />,
  Settings: (p) => <Icon {...p} d={<><circle cx="8" cy="8" r="2"/><path d="M8 1.5v1.8M8 12.7v1.8M14.5 8h-1.8M3.3 8H1.5M12.6 3.4l-1.3 1.3M4.7 11.3l-1.3 1.3M12.6 12.6l-1.3-1.3M4.7 4.7L3.4 3.4"/></>} />,
  Key: (p) => <Icon {...p} d={<><circle cx="5.5" cy="8" r="3"/><path d="M8 8h6M11 8v2M13 8v3"/></>} />,
  Webhook: (p) => <Icon {...p} d={<><circle cx="5" cy="5" r="2"/><circle cx="11" cy="11" r="2"/><circle cx="11" cy="5" r="2"/><path d="M6.5 6.5L9.5 9.5"/></>} />,
  Plus: (p) => <Icon {...p} d="M8 3v10M3 8h10" />,
  Check: (p) => <Icon {...p} d="M3 8.5l3 3 7-7" />,
  X: (p) => <Icon {...p} d="M3.5 3.5l9 9M12.5 3.5l-9 9" />,
  Chevron: (p) => <Icon {...p} d="M6 4l4 4-4 4" />,
  ChevronDown: (p) => <Icon {...p} d="M4 6l4 4 4-4" />,
  Folder: (p) => <Icon {...p} d={<><path d="M2 4.5a1 1 0 0 1 1-1h3l1.5 1.5H13a1 1 0 0 1 1 1V12a1 1 0 0 1-1 1H3a1 1 0 0 1-1-1z"/></>} />,
  File: (p) => <Icon {...p} d={<><path d="M3.5 2h6L13 5.5V13a1 1 0 0 1-1 1H3.5a1 1 0 0 1-1-1V3a1 1 0 0 1 1-1z"/><path d="M9 2v3.5h4"/></>} />,
  Branch: (p) => <Icon {...p} d={<><circle cx="4" cy="3.5" r="1.5"/><circle cx="4" cy="12.5" r="1.5"/><circle cx="12" cy="3.5" r="1.5"/><path d="M4 5v6M12 5v2a3 3 0 0 1-3 3H4"/></>} />,
  Commit: (p) => <Icon {...p} d={<><circle cx="8" cy="8" r="2.5"/><path d="M1.5 8H5.5M10.5 8H14.5"/></>} />,
  Star: (p) => <Icon {...p} d="M8 2l1.8 3.8 4.2.6-3 3 .8 4.2L8 11.6l-3.8 2L5 9.4l-3-3 4.2-.6z" />,
  Eye: (p) => <Icon {...p} d={<><path d="M1.5 8s2.5-4.5 6.5-4.5S14.5 8 14.5 8 12 12.5 8 12.5 1.5 8 1.5 8z"/><circle cx="8" cy="8" r="1.7"/></>} />,
  Fork: (p) => <Icon {...p} d={<><circle cx="4" cy="3" r="1.5"/><circle cx="12" cy="3" r="1.5"/><circle cx="8" cy="13" r="1.5"/><path d="M4 4.5v2a2 2 0 0 0 2 2h4a2 2 0 0 0 2-2v-2M8 8.5v3"/></>} />,
  Logo: (p) => <Icon {...p} d={<><path d="M3 8a5 5 0 0 1 10 0"/><path d="M3 8a5 5 0 0 0 10 0"/><circle cx="8" cy="8" r="1.5" fill="currentColor" stroke="none"/></>} />,
  Bolt: (p) => <Icon {...p} d="M9 1.5L3 9h4l-1 5.5L13 6H9z" />,
  Sun: (p) => <Icon {...p} d={<><circle cx="8" cy="8" r="3"/><path d="M8 1v1.5M8 13.5V15M1 8h1.5M13.5 8H15M3 3l1 1M12 12l1 1M3 13l1-1M12 4l1-1"/></>} />,
  Moon: (p) => <Icon {...p} d="M13 9a5 5 0 0 1-6.5-6 5.5 5.5 0 1 0 6.5 6z" />,
  Copy: (p) => <Icon {...p} d={<><rect x="5" y="5" width="9" height="9" rx="1.5"/><path d="M11 5V3.5A1.5 1.5 0 0 0 9.5 2H3.5A1.5 1.5 0 0 0 2 3.5v6A1.5 1.5 0 0 0 3.5 11H5"/></>} />,
  Split: (p) => <Icon {...p} d={<><rect x="1.5" y="2.5" width="13" height="11" rx="1.5"/><path d="M8 2.5v11"/></>} />,
  Close: (p) => <Icon {...p} d="M4 4l8 8M12 4l-8 8" />,
  Code: (p) => <Icon {...p} d="M5 5L2 8l3 3M11 5l3 3-3 3M9.5 3l-3 10" />,
  Book: (p) => <Icon {...p} d={<><path d="M2.5 3.5A1 1 0 0 1 3.5 2.5H8v11H3.5a1 1 0 0 1-1-1z"/><path d="M13.5 3.5a1 1 0 0 0-1-1H8v11h4.5a1 1 0 0 0 1-1z"/></>} />,
  Activity: (p) => <Icon {...p} d="M1.5 8h2.5l2-5 4 10 2-5h2.5" />,
  Bell: (p) => <Icon {...p} d={<><path d="M3.5 11.5L4 10V7a4 4 0 0 1 8 0v3l.5 1.5z"/><path d="M6.5 13a1.5 1.5 0 0 0 3 0"/></>} />,
  Filter: (p) => <Icon {...p} d="M2 3.5h12l-4.5 6V14L6.5 12V9.5z" />,
  Diff: (p) => <Icon {...p} d={<><path d="M4 2v10M2 4h4M11 4v8M9 12h4"/></>} />,
  Edit: (p) => <Icon {...p} d={<><path d="M10.5 2.5l3 3L6 13l-3.5 1 1-3.5z"/><path d="M9 4l3 3"/></>} />,
  Pin: (p) => <Icon {...p} d="M10 2l4 4-1.5 1.5-1-1L8 10.5l-2 .5-1 3 3-1 .5-2L11.5 7l-1-1z" />,
  Tag: (p) => <Icon {...p} d={<><path d="M2 2.5h5L14 9.5a1.5 1.5 0 0 1 0 2L11.5 14a1.5 1.5 0 0 1-2 0L2.5 7z"/><circle cx="5" cy="5" r="0.8" fill="currentColor"/></>} />,
};

window.Icon = Icon;
window.Icons = Icons;
