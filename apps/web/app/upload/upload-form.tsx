"use client";

import { FormEvent, useEffect, useState } from "react";

type UploadResult = {
  id: string;
  fileName: string;
  contentType: string;
  status: string;
};

type ErrorResponse = {
  error?: string;
};

const apiBaseUrl = process.env.NEXT_PUBLIC_RIVO_API_URL ?? "http://127.0.0.1:8080";

// Uploads are attributable, so the browser has to present a token. It is kept in
// sessionStorage rather than an environment variable: NEXT_PUBLIC_ values are baked
// into the client bundle, which is the wrong place for a credential even in
// development. This whole field disappears once real accounts exist.
const tokenStorageKey = "rivo.dev.apiToken";

export default function UploadForm() {
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [apiToken, setApiToken] = useState("");
  const [isUploading, setIsUploading] = useState(false);
  const [uploadResult, setUploadResult] = useState<UploadResult | null>(null);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  useEffect(() => {
    setApiToken(window.sessionStorage.getItem(tokenStorageKey) ?? "");
  }, []);

  function handleTokenChange(value: string) {
    setApiToken(value);
    window.sessionStorage.setItem(tokenStorageKey, value);
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();

    if (!selectedFile) {
      setErrorMessage("Choose a video before uploading.");
      return;
    }
    if (!apiToken.trim()) {
      setErrorMessage("Paste an API token before uploading.");
      return;
    }

    setIsUploading(true);
    setErrorMessage(null);
    setUploadResult(null);

    const formData = new FormData();
    formData.append("video", selectedFile);

    try {
      const response = await fetch(`${apiBaseUrl}/api/videos`, {
        method: "POST",
        headers: { Authorization: `Bearer ${apiToken.trim()}` },
        body: formData,
      });

      const payload = (await response.json()) as UploadResult | ErrorResponse;
      if (!response.ok) {
        const errorPayload = payload as ErrorResponse;
        throw new Error(errorPayload.error ?? "Rivo could not upload this video.");
      }

      setUploadResult(payload as UploadResult);
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : "Rivo could not upload this video.");
    } finally {
      setIsUploading(false);
    }
  }

  return (
    <form className="upload-form" onSubmit={handleSubmit}>
      <label className="token-field">
        <span>API token</span>
        <input
          autoComplete="off"
          name="apiToken"
          placeholder="rivo_dev_creator_token"
          type="password"
          value={apiToken}
          onChange={(event) => handleTokenChange(event.target.value)}
        />
        <small>Issued out of band. The development seed creates one.</small>
      </label>

      <label className="file-picker">
        <span>Video file</span>
        <input
          accept="video/*"
          name="video"
          type="file"
          onChange={(event) => setSelectedFile(event.target.files?.[0] ?? null)}
        />
      </label>

      {selectedFile ? (
        <p className="file-summary">
          {selectedFile.name} · {formatBytes(selectedFile.size)}
        </p>
      ) : null}

      <button
        className="primary-button"
        disabled={!selectedFile || !apiToken.trim() || isUploading}
        type="submit"
      >
        {isUploading ? "Uploading…" : "Upload video"}
      </button>

      <p className="upload-note">
        Default prototype limit: 1 GiB per request. The API identifies the container from
        the file itself, so an mp4 extension alone will not get a file accepted.
      </p>

      {errorMessage ? <p className="status-message error-message">{errorMessage}</p> : null}

      {uploadResult ? (
        <div className="status-message success-message">
          <strong>Stored locally.</strong>
          <span>Video ID: {uploadResult.id}</span>
          <span>
            {uploadResult.fileName} · detected as {uploadResult.contentType}
          </span>
        </div>
      ) : null}
    </form>
  );
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) {
    return `${bytes} B`;
  }

  const units = ["KiB", "MiB", "GiB", "TiB"];
  let value = bytes / 1024;
  let unitIndex = 0;

  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024;
    unitIndex += 1;
  }

  return `${value.toFixed(value >= 10 ? 1 : 2)} ${units[unitIndex]}`;
}
