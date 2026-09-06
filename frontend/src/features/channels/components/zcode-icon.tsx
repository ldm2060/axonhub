import React from 'react';

interface ZCodeIconProps {
  size?: number | string;
  className?: string;
  style?: React.CSSProperties;
}

// ZCode (z.ai) mark: deep-blue rounded square with a cyan "Z" monogram.
export const ZCodeIcon: React.FC<ZCodeIconProps> = ({ size = 20, className = '', style = {}, ...rest }) => {
  return (
    <svg
      height={size}
      style={{ flex: '0 0 auto', lineHeight: 1, ...style }}
      viewBox='0 0 144 144'
      width={size}
      xmlns='http://www.w3.org/2000/svg'
      className={className}
      {...rest}
    >
      <title>ZCode</title>
      <rect width='144' height='144' rx='34' fill='#1B2A4A' />
      <path d='M38 38h68v16L74 88h32v16H38V88l32-34H38V38z' fill='#7DE3FF' />
    </svg>
  );
};

export default ZCodeIcon;
