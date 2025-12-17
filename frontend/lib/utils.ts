import { type ClassValue, clsx } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

// frontend/lib/utils.ts
export const formatBytes = (bytes: any) => {
  const value = typeof bytes === 'string' ? parseInt(bytes) : bytes;
  if (!value || isNaN(value) || value === 0) return '0 B';

  // Assuming input in BYTES
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']; // Added PB for safety
  const i = Math.floor(Math.log(value) / Math.log(k));

  // Fail-safe for absurdly high values or index out of bounds
  if (i >= sizes.length || i < 0) {
      // Handle edge case by showing the largest unit
      return parseFloat((value / Math.pow(k, sizes.length - 1)).toFixed(2)) + ' ' + sizes[sizes.length - 1];
  }

  return parseFloat((value / Math.pow(k, i)).toFixed(2)) + ' ' + (sizes[i] || 'B');
};