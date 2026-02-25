"use client"

import Feedback from "../Feedback"

const Footer = () => {
  return (
    <footer className="border-t border-gray-200 dark:border-gray-800 py-8 px-4">
      <Feedback className="my-2" />
      <div className="flex gap-4 text-xs text-gray-400 mt-4">
        <a href="https://github.com/hanzoai/commerce" className="hover:text-gray-600">GitHub</a>
        <a href="https://hanzo.ai" className="hover:text-gray-600">hanzo.ai</a>
        <span>&copy; {new Date().getFullYear()} Hanzo Industries</span>
      </div>
    </footer>
  )
}

export default Footer
