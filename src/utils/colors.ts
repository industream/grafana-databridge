type BadgeColor = 'blue' | 'green' | 'orange' | 'red' | 'purple';

export function dataTypeColor(type: string): BadgeColor {
  switch (type) {
    case 'float64':
    case 'float32':
      return 'blue';
    case 'int64':
    case 'int32':
      return 'orange';
    case 'bool':
      return 'green';
    case 'string':
      return 'purple';
    case 'dateTime':
      return 'red';
    default:
      return 'blue';
  }
}

export function labelColor(label: string): BadgeColor {
  switch (label.toLowerCase()) {
    case 'analog': return 'blue';
    case 'digital': return 'green';
    case 'counter': return 'orange';
    case 'event': return 'purple';
    default: return 'blue';
  }
}
