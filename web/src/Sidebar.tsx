import React from 'react';
import './Sidebar.css';

interface SidebarBoxProps {
  logo: string;
  label?: string;
}

const SidebarBox: React.FC<SidebarBoxProps> = ({ logo, label }) => {
  return (
    <div className="sidebar-box">
      <img src={logo} alt={label} className="sidebar-icon" />
      {label && <div className="sidebar-label">{label}</div>}
    </div>
  );
};

const Sidebar: React.FC = () => {
  const boxes = [
    { logo: '/path/to/python-logo.png', label: 'Python' },
    { logo: '/path/to/r-logo.png', label: 'R' },
    { logo: '/path/to/javascript-logo.png', label: 'JavaScript' },
    { logo: '/path/to/java-logo.png', label: 'Java' },
  ];

  return (
    <div className="sidebar">
      {boxes.map((box, index) => (
        <SidebarBox key={index} logo={box.logo} label={box.label} />
      ))}
    </div>
  );
};

export default Sidebar;
