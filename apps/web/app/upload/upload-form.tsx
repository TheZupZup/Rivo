"use client";

import { FormEvent, useState } from "react";

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

export default function UploadForm() {
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [isUploading, setIsUploading] = useState(false);
  const [uploadResult, setUploadResult] = useState<UploadResult | null>(null);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();

    if (!selectedFile) {
      setErrorMessage("Choose a video before uploading.");
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

      <button className="primary-button" disabled={!selectedFile || isUploading} type="submit">
        {isUploading ? "Uploading…" : "Upload video"}
      </button>

      <p className="upload-note">Local prototype limit: 1 GiB per request.</p>

      {errorMessage ? <p className="status-message error-message">{errorMessage}</p> : null}

      {uploadResult ? (
        <div className="status-message success-message">
          <strong>Stored locally.</strong>
          <span>Video ID: {uploadResult.id}</span>
          <span>{uploadResult.fileName}</span>
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
