import { type KeyboardEvent, memo, type MouseEvent, type PointerEvent, useMemo, useRef, useState } from 'react'
import { useNavigate } from 'react-router'
import type { GroupBy, ImageGroup } from '../api'
import { groupLabel, headerText, headerTooltip, imageStatus, type StatusMap, textWidth, tooltipText, zoomLabel } from '../catalogMap'
import { focusOutline, freeStatusColor, groupHeaderColor } from '../styles'
import { type GroupLayout, layoutBlocks, layoutGroups, type Rect } from '../treemap'

const area: Rect = { x: 0, y: 0, w: 1000, h: 600 }
// Gaps separate the levels: 0.75 between units (their stroke), 2 between
// name-prefix blocks and 3 between groups (or between blocks when zoomed).
const gap = 3
const blockGap = 2
const header = 22

/**
 * What the treemap draws at its top level: the groups of the whole map, or
 * the blocks of a zoomed group. Each has a header label and its images.
 */
type Item = { label: string; images: string[] }

/**
 * The hovered unit (name set) or header (name empty), tied to the items it
 * was found in so a regrouped or rezoomed map drops it.
 */
type Tip = { name: string; item: number; x: number; y: number; items: Item[] }

/**
 * A treemap with a rectangle per group (variant kind or name prefix), each
 * with a header when it has room, divided into name-prefix blocks separated
 * by white space, with one equal unit per image colored by free-tier status.
 * Units are hidden from assistive technology and add no tab stops: the family
 * table offers every link by keyboard. Each group of the whole map is a
 * button, named for the group, that zooms into it; zoomed, the group's blocks
 * fill the map with a header each and nothing is focusable. Pointer users get
 * an immediate tooltip and can click a unit to open its image or a group
 * header to zoom in. Memoized, so sorting or paging the family table doesn't
 * redraw it.
 */
export const CatalogTreemap = memo(function CatalogTreemap({
  groups,
  groupBy,
  zoomed,
  statuses,
  summary,
  onZoom,
}: {
  groups: ImageGroup[]
  groupBy: GroupBy
  /** The group zoomed into, if any. */
  zoomed?: ImageGroup
  statuses: StatusMap
  summary: string
  onZoom: (label: string) => void
}) {
  const navigate = useNavigate()
  const svg = useRef<SVGSVGElement>(null)
  const [tip, setTip] = useState<Tip | null>(null)
  const items = useMemo<Item[]>(() => {
    if (!zoomed) return groups.map((g) => ({ label: groupLabel(g, groupBy), images: g.images }))
    let start = 0
    return zoomed.blocks.map((b) => {
      const images = zoomed.images.slice(start, start + b.count)
      start += b.count
      return { label: b.prefix, images }
    })
  }, [groups, groupBy, zoomed])
  // Layout depends only on the groups and zoom, so statuses arriving never move a unit.
  const layout = useMemo(
    () =>
      zoomed
        ? layoutBlocks(
            zoomed.blocks.map((b) => b.count),
            area,
            { gap, header },
          )
        : layoutGroups(
            groups.map((g) => g.blocks.map((b) => b.count)),
            area,
            { gap, header, blockGap },
          ),
    [groups, zoomed],
  )
  const zoomedLabel = zoomed && groupLabel(zoomed, groupBy)

  /** The unit or header under the pointer; a header has an empty name. */
  function hit(target: EventTarget) {
    const el = (target as Element).closest('[data-group]')
    return el && { name: el.getAttribute('data-image') ?? '', item: Number(el.getAttribute('data-group')) }
  }

  function onPointerMove(e: PointerEvent<SVGSVGElement>) {
    const found = hit(e.target)
    if (!found) {
      setTip(null)
      return
    }
    // Convert the pointer to SVG units so the tooltip follows it at any size.
    const ctm = svg.current?.getScreenCTM?.()
    const p = ctm ? new DOMPoint(e.clientX, e.clientY).matrixTransform(ctm.inverse()) : { x: 0, y: 0 }
    setTip({ ...found, x: p.x, y: p.y, items })
  }

  function onClick(e: MouseEvent<SVGSVGElement>) {
    const found = hit(e.target)
    if (!found) return
    if (found.name) navigate(`/images/${found.name}`)
    else if (!zoomed) onZoom(groups[found.item].label)
  }

  function tipText(t: Tip) {
    const item = items[t.item]
    if (t.name) return tooltipText(t.name, imageStatus(t.name, statuses), zoomedLabel ?? item.label)
    return headerTooltip(item.label, item.images.length, !zoomed)
  }

  return (
    <svg
      ref={svg}
      viewBox={`0 0 ${area.w} ${area.h}`}
      role={zoomed ? 'img' : 'group'}
      aria-label={summary}
      onPointerMove={onPointerMove}
      onPointerLeave={() => setTip(null)}
      onClick={onClick}
      className="block h-auto w-full cursor-pointer select-none"
    >
      <Items items={items} layout={layout} statuses={statuses} groups={zoomed ? undefined : groups} onZoom={onZoom} />
      {tip?.items === items && <Tooltip text={tipText(tip)} x={tip.x} y={tip.y} />}
    </svg>
  )
})

/**
 * The headers and units, kept apart from the tooltip so hovering doesn't
 * redraw them. With groups (the whole map), each item gets a zoom button
 * holding its header and a focus outline, drawn after its units.
 */
const Items = memo(function Items({
  items,
  layout,
  statuses,
  groups,
  onZoom,
}: {
  items: Item[]
  layout: GroupLayout[]
  statuses: StatusMap
  groups?: ImageGroup[]
  onZoom: (label: string) => void
}) {
  return layout.map(({ rect, header: band, units }, i) => {
    const { label, images } = items[i]
    const group = groups?.[i]
    // A group's header zooms in; a zoomed block's header does nothing.
    const headerBand = band && (
      <g data-group={i} data-header="" className={group ? 'cursor-pointer' : 'cursor-default'}>
        <rect x={band.x} y={band.y} width={band.w} height={band.h} className={groupHeaderColor.band} />
        <text x={band.x + 8} y={band.y + 15} fontSize="12" className={`font-medium ${groupHeaderColor.text}`}>
          {headerText(label, images.length, band.w)}
        </text>
      </g>
    )
    function onKeyDown(e: KeyboardEvent<SVGGElement>) {
      if (group && (e.key === 'Enter' || e.key === ' ')) {
        e.preventDefault()
        onZoom(group.label)
      }
    }
    return (
      <g key={label} data-group-label={label}>
        <g aria-hidden="true">
          {images.map((name, j) => {
            const u = units[j]
            return (
              <rect
                key={name}
                data-image={name}
                data-group={i}
                x={u.x}
                y={u.y}
                width={u.w}
                height={u.h}
                strokeWidth="0.75"
                className={`${freeStatusColor[imageStatus(name, statuses)].fill} stroke-white dark:stroke-zinc-900`}
              />
            )
          })}
          {!group && headerBand}
        </g>
        {group && (
          <g
            role="button"
            tabIndex={0}
            aria-label={zoomLabel(label, images.length)}
            data-zoom-group={group.label}
            onKeyDown={onKeyDown}
            className="group outline-none"
          >
            {headerBand}
            <rect
              x={rect.x + gap / 2}
              y={rect.y + gap / 2}
              width={Math.max(0, rect.w - gap)}
              height={Math.max(0, rect.h - gap)}
              strokeWidth="3"
              pointerEvents="none"
              data-focus-outline=""
              className={focusOutline}
            />
          </g>
        )}
      </g>
    )
  })
})

/** A tooltip drawn in the SVG near (x, y), flipped to stay inside the map. */
function Tooltip({ text, x, y }: { text: string; x: number; y: number }) {
  const w = textWidth(text) + 16
  const h = 24
  const left = Math.max(0, x + 12 + w > area.w ? x - 12 - w : x + 12)
  const top = Math.max(0, y + 12 + h > area.h ? y - 12 - h : y + 12)
  return (
    <g pointerEvents="none" data-tooltip="">
      <rect x={left} y={top} width={w} height={h} rx="4" className="fill-zinc-900 dark:fill-zinc-100" />
      <text x={left + 8} y={top + 16} fontSize="12" className="fill-white dark:fill-zinc-900">
        {text}
      </text>
    </g>
  )
}
