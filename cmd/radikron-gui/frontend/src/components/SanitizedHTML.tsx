import React from 'react';
import DOMPurify from 'dompurify';
import { Browser } from '@wailsio/runtime';

// SanitizedHTML renders radiko's program HTML with XSS protection and opens
// links in the system browser.
export const SanitizedHTML: React.FC<{ html: string; className?: string }> = ({ html, className }) => {
  const sanitizedHTML = DOMPurify.sanitize(html, {
    ALLOWED_TAGS: ['p', 'br', 'strong', 'em', 'u', 'a', 'ul', 'ol', 'li', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6'],
    ALLOWED_ATTR: ['href', 'target', 'rel'],
  });

  const handleClick = (e: React.MouseEvent<HTMLDivElement>) => {
    const link = (e.target as HTMLElement).closest('a');
    if (link && link.href) {
      e.preventDefault();
      Browser.OpenURL(link.href);
    }
  };

  return <div className={className} dangerouslySetInnerHTML={{ __html: sanitizedHTML }} onClick={handleClick} />;
};
