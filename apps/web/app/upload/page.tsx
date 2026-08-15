import Link from "next/link";
import UploadForm from "./upload-form";

export default function UploadPage() {
  return (
    <main className="page-shell compact-page-shell">
      <nav className="top-nav" aria-label="Primary navigation">
        <Link className="brand-link" href="/">
          Rivo
        </Link>
        <Link className="text-link" href="/">
          Back home
        </Link>
      </nav>

      <section className="upload-section">
        <p className="eyebrow">Local prototype</p>
        <h1 className="page-title">Upload the first Rivo video.</h1>
        <p className="intro">
          The source file is stored on your local Rivo instance. Transcoding and playback come next.
        </p>

        <div className="upload-card">
          <UploadForm />
        </div>
      </section>
    </main>
  );
}
