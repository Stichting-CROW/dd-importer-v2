#!/usr/bin/env python3
"""Bundle MDS 2.0 provider response schemas into standalone JSON Schema files.

MDS 2.0 is published as a split OpenAPI 3.1 document (openmobilityfoundation/mds-openapi)
with relative $refs between YAML files. The validator needs one self-contained
JSON Schema per endpoint, so this script inlines every reference.

Usage (requires PyYAML):

    git clone https://github.com/openmobilityfoundation/mds-openapi /tmp/mds-openapi
    python3 validator/schemas/bundle_mds2.py /tmp/mds-openapi validator/schemas/mds/2.0
"""
import json
import os
import sys

import yaml

ENDPOINTS = {
    'vehicles.json': '/vehicles',
    'vehicles_status.json': '/vehicles/status',
    'trips.json': '/trips',
}


def load(path, cache):
    if path not in cache:
        with open(path) as f:
            cache[path] = yaml.safe_load(f)
    return cache[path]


def resolve_pointer(doc, pointer):
    for part in [p for p in pointer.split('/') if p]:
        doc = doc[part.replace('~1', '/').replace('~0', '~')]
    return doc


def inline(node, base_file, cache, stack):
    if isinstance(node, list):
        return [inline(x, base_file, cache, stack) for x in node]
    if not isinstance(node, dict):
        return node

    if '$ref' in node:
        ref = node['$ref']
        file_part, _, pointer = ref.partition('#')
        target_file = os.path.normpath(
            os.path.join(os.path.dirname(base_file), file_part)) if file_part else base_file
        key = (target_file, pointer)
        if key in stack:
            raise ValueError(f'circular $ref {ref} in {base_file}')
        target = resolve_pointer(load(target_file, cache), pointer)
        resolved = inline(target, target_file, cache, stack | {key})
        siblings = {k: v for k, v in node.items() if k != '$ref'}
        if not siblings:
            return resolved
        return {'allOf': [resolved], **inline(siblings, base_file, cache, stack)}

    node = dict(node)
    # The mode models (car-share, micromobility, ...) overlap, so with oneOf a
    # vehicle that fits several modes is rejected. A record must fit one mode.
    if isinstance(node.get('oneOf'), list) and node['oneOf'] and all(
            isinstance(x, dict) and '/modes/' in x.get('$ref', '') for x in node['oneOf']):
        node['anyOf'] = node.pop('oneOf')

    result = {
        k: inline(v, base_file, cache, stack)
        for k, v in node.items()
        if not k.startswith('x-') and k != 'example'
    }
    # The mode event models write `contains: [a, b]`, which is not valid JSON
    # Schema; the intent is "contains one of these values".
    if isinstance(result.get('contains'), list):
        result['contains'] = {'enum': result['contains']}
    return result


def main(repo, out_dir):
    cache = {}
    spec_file = os.path.join(repo, 'reference', 'provider.yaml')
    spec = load(spec_file, cache)
    os.makedirs(out_dir, exist_ok=True)
    for file_name, path in ENDPOINTS.items():
        response = spec['paths'][path]['get']['responses']['200']
        schema = inline(response['content']['application/json']['schema'], spec_file, cache, set())
        schema = {
            '$schema': 'https://json-schema.org/draft/2020-12/schema',
            'title': f'MDS {spec["info"]["version"]} provider {path}',
            **schema,
        }
        with open(os.path.join(out_dir, file_name), 'w') as f:
            json.dump(schema, f, indent=1, sort_keys=True)
            f.write('\n')
        print(f'wrote {file_name} ({path})')


if __name__ == '__main__':
    if len(sys.argv) != 3:
        sys.exit(__doc__)
    main(sys.argv[1], sys.argv[2])
