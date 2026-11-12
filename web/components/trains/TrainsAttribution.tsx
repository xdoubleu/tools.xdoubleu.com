/** Attribution for the iRail API, which serves all train data. */
export default function TrainsAttribution() {
  return (
    <p className="text-xs text-muted">
      Source:{' '}
      <a
        href="https://irail.be"
        className="-my-4 inline-block min-w-11 py-4 text-center underline"
        target="_blank"
        rel="noreferrer"
      >
        iRail
      </a>
      , with data originally published by NMBS/SNCB. Modified by tools.xdoubleu.com.
    </p>
  )
}
