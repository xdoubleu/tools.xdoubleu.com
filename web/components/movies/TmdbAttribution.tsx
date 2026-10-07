/** Attribution required by TMDB's API terms. */
export default function TmdbAttribution() {
  return (
    <div className="mt-8 text-xs text-muted">
      <p>This product uses the TMDB API but is not endorsed or certified by TMDB.</p>
      <a
        href="https://www.themoviedb.org/"
        target="_blank"
        rel="noopener noreferrer"
        className="inline-flex min-h-11 items-center underline"
      >
        Metadata and posters: The Movie Database (TMDB)
      </a>
    </div>
  )
}
