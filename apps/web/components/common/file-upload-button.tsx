"use client";

import { useId } from "react";
import { Paperclip } from "lucide-react";
import { cn } from "@/lib/utils";

interface FileUploadButtonProps {
  /** Called with the selected File — caller handles upload. */
  onSelect: (file: File) => void;
  disabled?: boolean;
  className?: string;
  size?: "sm" | "default";
}

function FileUploadButton({
  onSelect,
  disabled,
  className,
  size = "default",
}: FileUploadButtonProps) {
  const inputId = useId();

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    e.target.value = "";
    onSelect(file);
  };

  const iconSize = size === "sm" ? "h-3.5 w-3.5" : "h-4 w-4";
  const btnSize = size === "sm" ? "h-6 w-6" : "h-7 w-7";

  return (
    <>
      <label
        htmlFor={inputId}
        aria-disabled={disabled || undefined}
        className={cn(
          "inline-flex cursor-pointer items-center justify-center rounded-full text-muted-foreground hover:bg-accent hover:text-foreground transition-colors disabled:opacity-50 disabled:pointer-events-none",
          disabled && "pointer-events-none opacity-50",
          btnSize,
          className,
        )}
      >
        <Paperclip className={iconSize} />
      </label>
      <input
        id={inputId}
        type="file"
        className="sr-only"
        tabIndex={-1}
        aria-hidden="true"
        disabled={disabled}
        onChange={handleChange}
      />
    </>
  );
}

export { FileUploadButton, type FileUploadButtonProps };
