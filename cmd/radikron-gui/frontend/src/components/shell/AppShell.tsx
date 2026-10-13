import React, { useEffect, useRef } from 'react';
import { useAppStore } from '@/store/useAppStore';
import { nextDownloads } from '@/lib/downloads';
import { formatCheckTime, statusShortLabel } from '@/lib/status';
import { screens, type ScreenId } from '@/lib/navigation';
import { ThemeToggle } from '@/components/ThemeToggle';
import { UpdateButton } from '@/components/UpdateButton';
import { cn } from '@/lib/utils';

interface AppShellProps {
  screen: ScreenId;
  onNavigate: (screen: ScreenId) => void;
  children: React.ReactNode;
}

// AppMark is the app icon without its tile: three blades around a star. The
// paths come from build/appicon.svg; the star follows the text color.
const AppMark: React.FC = () => (
  <svg width="28" height="28" viewBox="165 165 694 694" aria-hidden="true">
    <defs>
      <linearGradient id="app-mark-blades" x1="0" y1="0" x2="1" y2="1">
        <stop offset="0" stopColor="#F7B733" />
        <stop offset="1" stopColor="#E8590C" />
      </linearGradient>
    </defs>
    <path
      fill="url(#app-mark-blades)"
      d="M442 391 451 382 460 373 471 365 483 359 495 353 508 348 522 344 537 341 552 340 568 340 583 342 599 345 615 349 631 355 646 362 661 371 676 382 689 394 702 407 714 421 725 437 734 454 742 472 749 491 754 511 757 532 758 553 758 574 756 596 752 617 746 639 738 660 728 681 716 701 702 721 686 739 669 756 650 772 629 786 607 799 584 810 559 819 534 826 507 831 480 833 453 833 425 831 397 827 397 827 425 831 453 832 481 831 508 827 534 821 560 812 584 801 606 789 628 774 647 758 665 740 681 721 694 701 706 679 715 658 722 635 726 613 728 590 729 568 726 546 722 525 716 506 707 487 697 469 686 454 673 439 659 427 644 417 628 409 612 402 596 398 580 396 564 396 549 398 535 402 522 407 511 412 500 413 488 415 477 418 467 423 457 429 447 436 439 444 431 453 425 463 419 474 415 486ZM652 512 656 524 658 537 659 550 660 563 659 577 656 591 652 605 647 619 641 633 633 646 624 659 613 671 602 683 589 693 575 703 559 711 543 719 526 725 508 729 490 732 470 733 451 733 431 731 412 727 392 722 373 714 354 705 335 694 318 681 301 667 285 651 271 633 258 614 246 594 236 572 228 549 222 526 218 501 216 476 216 451 218 425 222 399 229 374 238 349 250 324 263 300 279 277 297 255 297 255 279 278 264 301 252 326 241 351 234 377 228 403 226 429 225 455 227 481 232 506 238 530 247 554 258 575 270 596 285 615 300 632 318 647 336 660 355 671 375 680 395 687 416 692 436 694 456 694 476 692 494 688 512 681 529 674 544 664 557 653 569 641 578 629 586 615 592 601 596 587 598 573 599 561 604 551 608 540 611 529 612 518 612 506 610 494 608 483 603 471 598 461 591 451 583 441ZM442 633 430 630 418 626 406 621 394 614 382 606 371 597 361 587 352 576 343 563 335 550 329 535 323 520 319 504 317 488 315 471 316 453 317 436 321 418 326 400 333 382 341 365 351 349 363 333 376 317 390 303 407 290 424 278 443 268 463 259 483 252 505 246 528 242 551 241 574 241 598 243 622 248 645 254 668 263 691 273 713 286 734 301 754 318 773 336 790 357 806 379 820 402 832 428 842 454 842 454 831 428 819 403 804 380 787 358 768 338 748 321 727 305 704 292 681 281 657 272 633 266 609 262 584 260 560 261 537 264 514 269 492 276 472 285 452 296 435 309 419 323 405 339 392 355 382 373 375 391 369 409 365 427 364 446 364 463 367 480 372 496 378 511 386 525 395 537 405 547 416 555 426 563 432 572 440 581 448 589 457 596 467 601 478 606 490 609 501 611 514 612 526 611 538 609Z"
    />
    <path fill="currentColor" d="M578 474 526 520 512 588 498 520 446 474 512 496ZM547 532 512 522 477 532 503 507 512 472 521 507Z" />
  </svg>
);

// SidebarStatus summarizes automatic downloads at the bottom of the sidebar.
const SidebarStatus: React.FC = () => {
  const status = useAppStore((state) => state.status);
  if (!status) {
    return null;
  }
  const label = statusShortLabel(status);
  const next = formatCheckTime(status.nextCheck);
  const detail = status.checking ? 'Checking now…' : next ? `Next check ${next}` : '';
  const failing = Boolean(status.configError || status.stationsError);

  return (
    <div className="rounded-[10px] border border-input p-3 flex flex-col gap-1.5" aria-live="polite">
      <div className="flex items-center gap-2">
        <span
          aria-hidden="true"
          className={cn(
            'size-2 rounded-full',
            status.monitoring ? 'bg-signal shadow-[0_0_0_4px_color-mix(in_srgb,var(--color-signal)_18%,transparent)]' : 'bg-muted-foreground',
          )}
        />
        <span className="font-medium">{label}</span>
      </div>
      {detail && <span className="text-xs text-muted-foreground">{detail}</span>}
      {failing && <span className="text-xs text-state-failed">Needs attention</span>}
    </div>
  );
};

// AppShell is the window frame: the sidebar with navigation, status, updates,
// and the theme switch, next to the current screen.
export const AppShell: React.FC<AppShellProps> = ({ screen, onNavigate, children }) => {
  const downloads = useAppStore((state) => state.downloads);
  const pending = nextDownloads(downloads, Number.POSITIVE_INFINITY).length;
  const counts: Partial<Record<ScreenId, number>> = { downloads: pending };
  const mainRef = useRef<HTMLElement>(null);

  // Each screen opens at its top.
  useEffect(() => {
    mainRef.current?.scrollTo?.({ top: 0 });
  }, [screen]);

  return (
    // Side by side, the sidebar stays put and only the screen scrolls; on a
    // narrow window the sidebar sits on top and the page scrolls as a whole.
    <div className="min-h-screen min-[780px]:h-screen min-[780px]:overflow-hidden flex flex-wrap bg-background text-foreground text-sm">
      <nav
        aria-label="Main"
        className="flex-[1_1_220px] max-w-60 min-[780px]:h-full min-[780px]:overflow-y-auto box-border px-3.5 py-5 bg-sidebar border-r border-border flex flex-col gap-1"
      >
        <div className="flex items-center gap-2.5 px-2.5 pt-1 pb-5 text-foreground">
          <AppMark />
          <span className="font-semibold text-[17px] text-foreground">Radikron</span>
        </div>
        {screens.map((item) => {
          const active = item.id === screen;
          const count = counts[item.id];
          return (
            <button
              key={item.id}
              type="button"
              aria-current={active ? 'page' : undefined}
              onClick={() => onNavigate(item.id)}
              className={cn(
                'flex items-center justify-between min-h-11 px-3 rounded-[10px] text-left cursor-pointer transition-colors',
                active ? 'bg-accent text-foreground font-medium' : 'text-muted-foreground hover:bg-accent/60 hover:text-foreground',
              )}
            >
              <span>{item.label}</span>
              {count ? <span className="font-mono text-xs text-muted-foreground">{count}</span> : null}
            </button>
          );
        })}
        <div className="grow min-h-6" />
        <div className="flex flex-col gap-2">
          <SidebarStatus />
          <UpdateButton />
          <ThemeToggle />
        </div>
      </nav>
      <main ref={mainRef} className="flex-[999_1_560px] min-w-0 min-[780px]:h-full min-[780px]:overflow-y-auto box-border px-8 pt-7 pb-10">
        {children}
      </main>
    </div>
  );
};
