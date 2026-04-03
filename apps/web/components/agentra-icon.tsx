import { useEffect, useState } from "react";
import { cn } from "@/lib/utils";

interface AgentraIconProps extends React.ComponentProps<"span"> {
  animate?: boolean;
  noSpin?: boolean;
  bordered?: boolean;
  size?: "sm" | "md" | "lg";
}

const borderedSizes = {
  sm: { wrapper: "p-1.5", icon: "size-3.5" },
  md: { wrapper: "p-2", icon: "size-4" },
  lg: { wrapper: "p-2.5", icon: "size-5" },
};

function AgentraMark({ className }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 100 100"
      className={cn("block size-full", className)}
      aria-hidden="true"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
    >
      <defs>
        <polygon
          id="agentra-blade"
          points="50,7 67.5,36 57,58 50,52 43,58 32.5,36"
        />
      </defs>
      <use href="#agentra-blade" fill="currentColor" />
      <use href="#agentra-blade" fill="currentColor" transform="rotate(120 50 50)" />
      <use href="#agentra-blade" fill="currentColor" transform="rotate(240 50 50)" />
      <circle cx="50" cy="50" r="7.5" fill="currentColor" />
    </svg>
  );
}

export function AgentraIcon({
  className,
  animate = false,
  noSpin = false,
  bordered = false,
  size = "sm",
  ...props
}: AgentraIconProps) {
  const [entranceDone, setEntranceDone] = useState(!animate);

  useEffect(() => {
    if (!animate) return;
    const timer = setTimeout(() => setEntranceDone(true), 600);
    return () => clearTimeout(timer);
  }, [animate]);

  const iconClassName = cn(
    !entranceDone && "animate-entrance-spin",
    entranceDone && !noSpin && "hover:animate-spin",
  );

  if (bordered) {
    const sizeConfig = borderedSizes[size];
    return (
      <span
        className={cn(
          "inline-flex items-center justify-center rounded-md border border-border",
          sizeConfig.wrapper,
          className,
        )}
        aria-hidden="true"
        {...props}
      >
        <span className={cn("block", sizeConfig.icon, iconClassName)}>
          <AgentraMark />
        </span>
      </span>
    );
  }

  return (
    <span
      className={cn("inline-block size-[1em]", className)}
      aria-hidden="true"
      {...props}
    >
      <span className={cn("block size-full", iconClassName)}>
        <AgentraMark />
      </span>
    </span>
  );
}
