import { useEffect, useEffectEvent } from "react";

export type UseIntersectionObserverOptions = {
  threshold?: number | number[];
  root?: Element | null;
  rootMargin?: string;
  disabled?: boolean;
};

export function useIntersectionObserver(
  targetRef: React.RefObject<Element | null>,
  onIntersecting: (entry: IntersectionObserverEntry) => void,
  {
    threshold = 0,
    root = null,
    rootMargin = "0px",
    disabled = false,
  }: UseIntersectionObserverOptions = {},
) {
  const onIntersectingEvent = useEffectEvent(onIntersecting);

  useEffect(() => {
    const el = targetRef.current;
    if (!el || disabled) return;

    const observer = new window.IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) {
            onIntersectingEvent(entry);
          }
        }
      },
      { threshold, root, rootMargin },
    );

    observer.observe(el);
    return () => observer.disconnect();
  }, [targetRef, threshold, root, rootMargin, disabled]);
}
