import { useEffect, RefObject } from 'react';

/**
 * Hook that alerts clicks outside of the passed ref(s)
 */
export function useClickOutside(
  refs: RefObject<HTMLElement | null> | RefObject<HTMLElement | null>[],
  handler: () => void
) {
  useEffect(() => {
    const listener = (event: MouseEvent | TouchEvent) => {
      const target = event.target as Node;
      
      const refsArray = Array.isArray(refs) ? refs : [refs];
      
      // Check if the click was inside any of the provided refs
      const isInside = refsArray.some(ref => ref.current && ref.current.contains(target));
      
      if (isInside) {
        return;
      }

      handler();
    };

    document.addEventListener('mousedown', listener);
    document.addEventListener('touchstart', listener);

    return () => {
      document.removeEventListener('mousedown', listener);
      document.removeEventListener('touchstart', listener);
    };
  }, [refs, handler]);
}
