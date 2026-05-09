interface PRStatusBadgeProps {
  prNumber?: number
  repository?: string
  status?: string
}

export function PRStatusBadge({ prNumber, repository, status }: PRStatusBadgeProps) {
  if (!prNumber) return null

  return (
    <div className="flex items-center gap-1 text-sm">
      <span className="text-muted-foreground">PR</span>
      <a
        href={`https://github.com/${repository}/pull/${prNumber}`}
        target="_blank"
        rel="noopener noreferrer"
        className="text-primary hover:underline"
      >
        #{prNumber}
      </a>
      {status && (
        <span className={`text-xs px-1 rounded ${
          status === 'open' ? 'bg-green-100 text-green-800' :
          status === 'closed' ? 'bg-red-100 text-red-800' :
          'bg-gray-100 text-gray-800'
        }`}>
          {status}
        </span>
      )}
    </div>
  )
}