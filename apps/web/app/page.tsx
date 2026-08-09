const principles = [
  "A report is not a conviction.",
  "New rules do not create retroactive creator strikes.",
  "Major moderation decisions must be explainable and appealable.",
  "The platform stays open source and contributor-friendly.",
];

export default function HomePage() {
  return (
    <main className="page-shell">
      <section className="hero">
        <p className="eyebrow">Working prototype</p>
        <h1>A video platform built around creator trust.</h1>
        <p className="intro">
          Open source, transparent moderation, versioned rules, and real appeals.
        </p>
      </section>

      <section className="principles" aria-labelledby="principles-title">
        <h2 id="principles-title">Foundation</h2>
        <ul>
          {principles.map((principle) => (
            <li key={principle}>{principle}</li>
          ))}
        </ul>
      </section>
    </main>
  );
}
