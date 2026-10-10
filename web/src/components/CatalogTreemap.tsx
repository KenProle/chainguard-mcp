import { memo, type MouseEvent, type PointerEvent, useMemo, useRef, useState } from 'react'
import { useNavigate } from 'react-router'
import type { GroupBy, ImageGroup } from '../api'
import { groupLabel, headerText, headerTooltip, imageStatus, type StatusMap, textWidth, tooltipText } from '../catalogMap'
import { freeStatusColor, groupHeaderColor } from '../styles'
import { type GroupLayout, layoutGroups, type Rect } from '../treemap'

const area: Rect = { x: 0, y: 0, w: 1000, h: 600 }
// Gaps separate the levels: 0.75 between units (their stroke), 2 between
// name-prefix blocks and 3 between groups.
const gap = 3
const blockGap = 2
const header = 22

/**
 * The hovered unit (name set) or group header (name empty), tied to the
 * groups it was found in so a regrouped map drops it.
 */
type Tip = { name: string; group: number; x: number; y: number; groups: ImageGroup[] }

/**
 * A treemap with a rectangle per group (variant kind or name prefix), each
 * with a header when it has room, divided into name-prefix blocks separated
 * by white space, with one equal unit per image colored by free-tier status. The SVG is one image to assistive technology, named by
 * summary, and adds no tab stops: the family table offers every link by
 * keyboard. Pointer users get an immediate tooltip and can click a unit to
 * open its image. Memoized, so sorting or paging the family table doesn't
 * redraw it.
 */
export const CatalogTreemap = memo(function CatalogTreemap({
  groups,
  groupBy,
  statuses,
  summary,
}: {
  groups: ImageGroup[]
  groupBy: GroupBy
  statuses: StatusMap
  summary: string
}) {
  const navigate = useNavigate()
  const svg = useRef<SVGSVGElement>(null)
  const [tip, setTip] = useState<Tip | null>(null)
  const labels = useMemo(() => groups.map((g) => groupLabel(g, groupBy)), [groups, groupBy])
  // Layout depends only on the groups, so statuses arriving never move a unit.
  const layout = useMemo(
    () =>
      layoutGroups(
        groups.map((g) => g.blocks.map((b) => b.count)),
        area,
        { gap, header, blockGap },
      ),
    [groups],
  )

  /** The unit or group header under the pointer; a header has an empty name. */
  function hit(target: EventTarget) {
    const el = (target as Element).closest('[data-group]')
    return el && { name: el.getAttribute('data-image') ?? '', group: Number(el.getAttribute('data-group')) }
  }

  function onPointerMove(e: PointerEvent<SVGSVGElement>) {
    const unit = hit(e.target)
    if (!unit) {
      setTip(null)
      return
    }
    // Convert the pointer to SVG units so the tooltip follows it at any size.
    const ctm = svg.current?.getScreenCTM?.()
    const p = ctm ? new DOMPoint(e.clientX, e.clientY).matrixTransform(ctm.inverse()) : { x: 0, y: 0 }
    setTip({ ...unit, x: p.x, y: p.y, groups })
  }

  function onClick(e: MouseEvent<SVGSVGElement>) {
    const unit = hit(e.target)
    if (unit?.name) navigate(`/images/${unit.name}`)
  }

  return (
    <svg
      ref={svg}
      viewBox={`0 0 ${area.w} ${area.h}`}
      role="img"
      aria-label={summary}
      onPointerMove={onPointerMove}
      onPointerLeave={() => setTip(null)}
      onClick={onClick}
      className="block h-auto w-full cursor-pointer select-none"
    >
      <Groups groups={groups} labels={labels} layout={layout} statuses={statuses} />
      {tip?.groups === groups && (
        <Tooltip
          text={
            tip.name
              ? tooltipText(tip.name, imageStatus(tip.name, statuses), labels[tip.group])
              : headerTooltip(labels[tip.group], groups[tip.group].images.length)
          }
          x={tip.x}
          y={tip.y}
        />
      )}
    </svg>
  )
})

/** The headers and units, kept apart from the tooltip so hovering doesn't redraw them. */
const Groups = memo(function Groups({
  groups,
  labels,
  layout,
  statuses,
}: {
  groups: ImageGroup[]
  labels: string[]
  layout: GroupLayout[]
  statuses: StatusMap
}) {
  return layout.map(({ header: band, units }, i) => (
    <g key={labels[i]} data-group-label={labels[i]}>
      {band && (
        <g data-group={i} data-header="">
          <rect x={band.x} y={band.y} width={band.w} height={band.h} className={groupHeaderColor.band} />
          <text x={band.x + 8} y={band.y + 15} fontSize="12" className={`font-medium ${groupHeaderColor.text}`}>
            {headerText(labels[i], groups[i].images.length, band.w)}
          </text>
        </g>
      )}
      {groups[i].images.map((name, j) => {
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
    </g>
  ))
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
