import { describe, it, expect } from "vitest"
import { npxToYarn } from "../npx-to-yarn.js"

describe("npxToYarn", () => {
  describe("yarn conversion", () => {
    it("should convert basic npx command to yarn", () => {
      const result = npxToYarn("npx hanzo-commerce db:migrate", "yarn")
      expect(result).toBe("yarn hanzo-commerce db:migrate")
    })

    it("should convert npx command with multiple arguments", () => {
      const result = npxToYarn("npx hanzo-commerce develop --port 9000", "yarn")
      expect(result).toBe("yarn hanzo-commerce develop --port 9000")
    })

    it("should convert npx command with flags", () => {
      const result = npxToYarn("npx hanzo-commerce user --email admin@test.com", "yarn")
      expect(result).toBe("yarn hanzo-commerce user --email admin@test.com")
    })

    it("should handle npx command with leading/trailing whitespace", () => {
      const result = npxToYarn("  npx hanzo-commerce db:migrate  ", "yarn")
      expect(result).toBe("yarn hanzo-commerce db:migrate")
    })

    it("should convert npx command to yarn dlx when isExecutable is true", () => {
      const result = npxToYarn("npx create-hanzo-app@latest", "yarn", true)
      expect(result).toBe("yarn dlx create-hanzo-app@latest")
    })

    it("should convert npx command to yarn when isExecutable is false", () => {
      const result = npxToYarn("npx hanzo-commerce db:migrate", "yarn", false)
      expect(result).toBe("yarn hanzo-commerce db:migrate")
    })
  })

  describe("pnpm conversion", () => {
    it("should convert basic npx command to pnpm", () => {
      const result = npxToYarn("npx hanzo-commerce db:migrate", "pnpm")
      expect(result).toBe("pnpm hanzo-commerce db:migrate")
    })

    it("should convert npx command with multiple arguments", () => {
      const result = npxToYarn("npx hanzo-commerce develop --port 9000", "pnpm")
      expect(result).toBe("pnpm hanzo-commerce develop --port 9000")
    })

    it("should convert npx command with flags", () => {
      const result = npxToYarn("npx hanzo-commerce user --email admin@test.com", "pnpm")
      expect(result).toBe("pnpm hanzo-commerce user --email admin@test.com")
    })

    it("should handle npx command with leading/trailing whitespace", () => {
      const result = npxToYarn("  npx hanzo-commerce db:migrate  ", "pnpm")
      expect(result).toBe("pnpm hanzo-commerce db:migrate")
    })

    it("should convert npx command to pnpm dlx when isExecutable is true", () => {
      const result = npxToYarn("npx create-hanzo-app@latest", "pnpm", true)
      expect(result).toBe("pnpm dlx create-hanzo-app@latest")
    })

    it("should convert npx command to pnpm when isExecutable is false", () => {
      const result = npxToYarn("npx hanzo-commerce db:migrate", "pnpm", false)
      expect(result).toBe("pnpm hanzo-commerce db:migrate")
    })
  })

  describe("edge cases", () => {
    it("should return original command if it does not start with npx", () => {
      const result = npxToYarn("npm install hanzo-commerce", "yarn")
      expect(result).toBe("npm install hanzo-commerce")
    })

    it("should handle command with only npx and package name", () => {
      const result = npxToYarn("npx hanzo-commerce", "yarn")
      expect(result).toBe("yarn hanzo-commerce")
    })

    it("should preserve command structure with special characters", () => {
      const result = npxToYarn("npx hanzo-commerce db:seed --file=./data.json", "pnpm")
      expect(result).toBe("pnpm hanzo-commerce db:seed --file=./data.json")
    })

    it("should handle command with path separators", () => {
      const result = npxToYarn("npx @medusajs/hanzo-cli develop", "yarn")
      expect(result).toBe("yarn @medusajs/hanzo-cli develop")
    })

    it("should handle multi-line commands with backslash continuation", () => {
      const multiLineCommand = `npx create-hanzo-app@latest \\
  --db-url postgres://localhost/hanzo \\
  --skip-db`
      const result = npxToYarn(multiLineCommand, "yarn", true)
      expect(result).toBe(`yarn dlx create-hanzo-app@latest \\
  --db-url postgres://localhost/hanzo \\
  --skip-db`)
    })

    it("should handle multi-line commands for pnpm", () => {
      const multiLineCommand = `npx hanzo-commerce develop \\
  --port 9000 \\
  --verbose`
      const result = npxToYarn(multiLineCommand, "pnpm")
      expect(result).toBe(`pnpm hanzo-commerce develop \\
  --port 9000 \\
  --verbose`)
    })

    it("should handle commands with newlines", () => {
      const commandWithNewlines = "npx create-hanzo-app@latest\n  --db-url postgres://localhost/hanzo"
      const result = npxToYarn(commandWithNewlines, "yarn", true)
      expect(result).toBe("yarn dlx create-hanzo-app@latest\n  --db-url postgres://localhost/hanzo")
    })

    it("should convert multiple npx commands on separate lines for yarn", () => {
      const multipleCommands = `npx hanzo-commerce db:migrate
npx hanzo-commerce develop`
      const result = npxToYarn(multipleCommands, "yarn")
      expect(result).toBe(`yarn hanzo-commerce db:migrate
yarn hanzo-commerce develop`)
    })

    it("should convert multiple npx commands on separate lines for pnpm", () => {
      const multipleCommands = `npx hanzo-commerce db:migrate
npx hanzo-commerce user --email admin@test.com
npx hanzo-commerce develop --port 9000`
      const result = npxToYarn(multipleCommands, "pnpm")
      expect(result).toBe(`pnpm hanzo-commerce db:migrate
pnpm hanzo-commerce user --email admin@test.com
pnpm hanzo-commerce develop --port 9000`)
    })

    it("should convert multiple npx commands with executable flag", () => {
      const multipleCommands = `npx create-hanzo-app@latest
npx @medusajs/hanzo-cli init`
      const result = npxToYarn(multipleCommands, "yarn", true)
      expect(result).toBe(`yarn dlx create-hanzo-app@latest
yarn dlx @medusajs/hanzo-cli init`)
    })

    it("should preserve indentation when converting multiple commands", () => {
      const indentedCommands = `npx hanzo-commerce db:migrate
  npx hanzo-commerce develop
    npx hanzo-commerce user`
      const result = npxToYarn(indentedCommands, "pnpm")
      expect(result).toBe(`pnpm hanzo-commerce db:migrate
  pnpm hanzo-commerce develop
    pnpm hanzo-commerce user`)
    })

    it("should handle mixed npx and non-npx lines", () => {
      const mixedCommands = `npx hanzo-commerce db:migrate
echo "Migration complete"
npx hanzo-commerce develop`
      const result = npxToYarn(mixedCommands, "yarn")
      expect(result).toBe(`yarn hanzo-commerce db:migrate
echo "Migration complete"
yarn hanzo-commerce develop`)
    })
  })
})
