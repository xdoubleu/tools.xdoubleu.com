import TmdbLogo from '@/components/movies/TmdbLogo'

/** Credits block required by TMDB's API terms (logo and notice). */
export default function TmdbAttribution() {
  return (
    <section aria-labelledby="tmdb-credits" className="mt-8 text-xs text-muted">
      <h2 id="tmdb-credits" className="mb-2 font-semibold">
        Credits
      </h2>
      <a
        href="https://www.themoviedb.org/"
        target="_blank"
        rel="noopener noreferrer"
        className="inline-flex min-h-11 items-center"
      >
        <TmdbLogo className="h-3 w-auto" />
      </a>
      <p>
        This application uses TMDB and the TMDB APIs but is not endorsed, certified, or otherwise
        approved by TMDB. Metadata and posters: The Movie Database (TMDB).
      </p>
    </section>
  )
}
