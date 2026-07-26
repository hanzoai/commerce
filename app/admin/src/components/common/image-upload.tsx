'use client'

import { useCallback, useRef, useState } from 'react'
import { Button, IconButton, Text, toast, clx } from '@hanzo/commerce-ui'
import { ArrowUpTray, Photo, Spinner, Trash } from '@hanzo/commerce-icons'
import { useUploadImages } from '@/lib/api/hooks'

// The image MIME types the backend accepts (byte-sniffed there — this is only
// the picker filter + a friendly client-side pre-check).
const ACCEPT = 'image/jpeg,image/png,image/gif,image/webp'
const ACCEPT_SET = new Set(ACCEPT.split(','))
const MAX_BYTES = 10 * 1024 * 1024 // mirrors upload.MaxUploadBytes (10 MiB)

interface ImageUploadProps {
  /** Current image URL(s). Always an array; single-image callers pass 0 or 1. */
  value: string[]
  /** Called with the next URL array after an upload or a remove. */
  onChange: (urls: string[]) => void
  /** Allow more than one image (gallery). Default true. */
  multiple?: boolean
  /** Disable all interaction (read-only form). */
  disabled?: boolean
  className?: string
}

// ImageUpload posts image file(s) to POST /v1/upload/images and stores the
// returned public CDN URLs in the form via onChange. It shows a live thumbnail
// grid with remove buttons, drag-and-drop, and an in-flight spinner. One control
// serves both the single-image (thumbnail/banner) and multi-image (gallery)
// cases via `multiple`.
export function ImageUpload({ value, onChange, multiple = true, disabled, className }: ImageUploadProps) {
  const upload = useUploadImages()
  const inputRef = useRef<HTMLInputElement>(null)
  const [dragOver, setDragOver] = useState(false)

  const urls = value ?? []
  const busy = upload.isPending

  const ingest = useCallback(
    async (fileList: FileList | null) => {
      if (!fileList || fileList.length === 0) return
      let files = Array.from(fileList)

      // Reject anything the backend would reject, with a clear reason, before
      // spending an upload round-trip.
      const bad = files.find((f) => !ACCEPT_SET.has(f.type) || f.size > MAX_BYTES)
      if (bad) {
        toast.error(
          !ACCEPT_SET.has(bad.type)
            ? `${bad.name}: only JPEG, PNG, GIF, or WebP images are allowed`
            : `${bad.name} is larger than 10MB`,
        )
        return
      }

      // A single-image control keeps only the last picked file.
      if (!multiple) files = files.slice(-1)

      try {
        const uploaded = await upload.mutateAsync(files)
        onChange(multiple ? [...urls, ...uploaded] : uploaded.slice(-1))
        toast.success(uploaded.length > 1 ? `${uploaded.length} images uploaded` : 'Image uploaded')
      } catch (e) {
        toast.error(e instanceof Error ? e.message : 'Upload failed')
      }
    },
    [multiple, onChange, upload, urls],
  )

  const remove = (url: string) => onChange(urls.filter((u) => u !== url))

  const onDrop = (e: React.DragEvent) => {
    e.preventDefault()
    setDragOver(false)
    if (disabled || busy) return
    void ingest(e.dataTransfer.files)
  }

  const canAddMore = multiple || urls.length === 0

  return (
    <div className={clx('flex flex-col gap-y-3', className)}>
      {urls.length > 0 && (
        <div className="grid grid-cols-3 gap-3 sm:grid-cols-4">
          {urls.map((url) => (
            <div
              key={url}
              className="group relative aspect-square overflow-hidden rounded-lg border border-ui-border-base bg-ui-bg-subtle"
            >
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img src={url} alt="" className="size-full object-cover" />
              {!disabled && (
                <IconButton
                  type="button"
                  size="2xsmall"
                  variant="transparent"
                  onClick={() => remove(url)}
                  aria-label="Remove image"
                  className="absolute right-1 top-1 bg-ui-bg-base/80 opacity-0 transition-opacity group-hover:opacity-100"
                >
                  <Trash className="text-ui-fg-error" />
                </IconButton>
              )}
            </div>
          ))}
        </div>
      )}

      {!disabled && canAddMore && (
        <div
          role="button"
          tabIndex={0}
          onClick={() => !busy && inputRef.current?.click()}
          onKeyDown={(e) => {
            if ((e.key === 'Enter' || e.key === ' ') && !busy) inputRef.current?.click()
          }}
          onDragOver={(e) => {
            e.preventDefault()
            setDragOver(true)
          }}
          onDragLeave={() => setDragOver(false)}
          onDrop={onDrop}
          className={clx(
            'flex cursor-pointer flex-col items-center justify-center gap-y-2 rounded-lg border border-dashed border-ui-border-strong px-4 py-6 text-center transition-colors',
            dragOver && 'border-ui-fg-interactive bg-ui-bg-highlight',
            busy && 'pointer-events-none opacity-70',
          )}
        >
          {busy ? (
            <Spinner className="animate-spin text-ui-fg-muted" />
          ) : urls.length > 0 ? (
            <ArrowUpTray className="text-ui-fg-muted" />
          ) : (
            <Photo className="text-ui-fg-muted" />
          )}
          <Text size="small" className="text-ui-fg-subtle">
            {busy ? 'Uploading…' : (
              <>
                <span className="text-ui-fg-interactive">Click to upload</span> or drag and drop
              </>
            )}
          </Text>
          <Text size="xsmall" className="text-ui-fg-muted">
            JPEG, PNG, GIF, or WebP — up to 10MB{multiple ? ' each' : ''}
          </Text>
        </div>
      )}

      {!disabled && !canAddMore && (
        <Button
          type="button"
          variant="secondary"
          size="small"
          isLoading={busy}
          onClick={() => inputRef.current?.click()}
        >
          Replace image
        </Button>
      )}

      <input
        ref={inputRef}
        type="file"
        accept={ACCEPT}
        multiple={multiple}
        className="hidden"
        disabled={disabled || busy}
        onChange={(e) => {
          void ingest(e.target.files)
          e.target.value = '' // allow re-selecting the same file
        }}
      />
    </div>
  )
}
