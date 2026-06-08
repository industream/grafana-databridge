import { Transform } from '../types';
import { getTransformParams, setTransformInPipeline } from './transforms';

describe('setTransformInPipeline', () => {
  it('adds a transform in wrapper-object shape', () => {
    const next = setTransformInPipeline(undefined, 'movingAverage', { window: 5 });
    expect(next).toEqual([{ movingAverage: { window: 5 } }]);
  });

  it('keeps transforms in fixed pipeline order regardless of insertion order', () => {
    let pipeline: Transform[] = [];
    pipeline = setTransformInPipeline(pipeline, 'rollingStats', { window: 10 });
    pipeline = setTransformInPipeline(pipeline, 'resample', { every: 'PT1M', aggregation: 'mean' });
    pipeline = setTransformInPipeline(pipeline, 'fill', { method: 'linear' });

    const kinds = pipeline.map((t) => Object.keys(t)[0]);
    expect(kinds).toEqual(['resample', 'fill', 'rollingStats']);
  });

  it('replaces an existing transform of the same kind', () => {
    let pipeline = setTransformInPipeline(undefined, 'resample', { every: 'PT1M' });
    pipeline = setTransformInPipeline(pipeline, 'resample', { every: 'PT5M', aggregation: 'max' });

    expect(pipeline).toHaveLength(1);
    expect(pipeline[0].resample).toEqual({ every: 'PT5M', aggregation: 'max' });
  });

  it('removes a transform when params is undefined', () => {
    let pipeline = setTransformInPipeline(undefined, 'cumulativeSum', {});
    pipeline = setTransformInPipeline(pipeline, 'movingAverage', { window: 3 });
    pipeline = setTransformInPipeline(pipeline, 'cumulativeSum', undefined);

    expect(pipeline.map((t) => Object.keys(t)[0])).toEqual(['movingAverage']);
  });
});

describe('getTransformParams', () => {
  it('returns the params of a present transform', () => {
    const transforms: Transform[] = [{ resample: { every: 'PT1M', aggregation: 'mean' } }];
    expect(getTransformParams(transforms, 'resample')).toEqual({ every: 'PT1M', aggregation: 'mean' });
  });

  it('returns undefined when the kind is absent', () => {
    expect(getTransformParams([{ fill: { method: 'linear' } }], 'resample')).toBeUndefined();
    expect(getTransformParams(undefined, 'resample')).toBeUndefined();
  });
});
