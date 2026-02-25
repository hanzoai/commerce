import { Button } from "@hanzo/commerce-ui"

export default function ButtonAsLink() {
  return (
    <Button asChild>
      <a href="https://hanzo.ai" target="_blank" rel="noopener noreferrer">
        Open Hanzo Commerce Website
      </a>
    </Button>
  )
}
