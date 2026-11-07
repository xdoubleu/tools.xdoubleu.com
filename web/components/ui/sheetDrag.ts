/** Distance past which a released drag dismisses, as a fraction of sheet height. */
const DISMISS_FRACTION = 0.4
/** Downward release speed (px/ms) that counts as a fling. */
const FLING_VELOCITY = 0.5

/** Decides whether a released sheet drag closes it: a downward fling or a long enough pull. */
function shouldDismissSheet(offset: number, velocity: number, height: number): boolean {
  if (offset <= 0) return false
  if (velocity >= FLING_VELOCITY) return true
  return height > 0 && offset / height >= DISMISS_FRACTION
}

/** Px/ms between two samples; 0 when no time has passed. */
function dragVelocity(deltaPx: number, deltaMs: number): number {
  return deltaMs > 0 ? deltaPx / deltaMs : 0
}

export { shouldDismissSheet, dragVelocity, DISMISS_FRACTION, FLING_VELOCITY }
