/**
 * The scrollLeft that brings an item fully into view in a horizontally
 * scrolling container, moving as little as possible. Positions are relative to
 * the start of the container's content.
 */
export function scrollLeftToReveal(scrollLeft: number, viewWidth: number, itemLeft: number, itemWidth: number): number {
  if (itemLeft < scrollLeft) return itemLeft
  if (itemLeft + itemWidth > scrollLeft + viewWidth) return Math.min(itemLeft, itemLeft + itemWidth - viewWidth)
  return scrollLeft
}
