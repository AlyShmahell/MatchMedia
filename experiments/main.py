import math
import os
import re
from collections import defaultdict

import numpy as np
from sklearn.cluster import AgglomerativeClustering
from sklearn.feature_extraction.text import TfidfVectorizer
from sklearn.metrics.pairwise import cosine_distances
from sklearn.preprocessing import normalize

ROOT_DIR = ""
PARENT_WEIGHT = 1
DISTANCE_THRESHOLD = 0.49
VIDEO_EXTENSIONS = {".mkv", ".mp4", ".avi", ".m4v", ".ts", ".m2ts"}
SEASON_RE = re.compile(
    r"(?i)^(season\s*\d+|ovas?|specials?|movies?|others?|extras?|s\d{1,2})$"
)


def is_video(name):
    return os.path.splitext(name)[1].lower() in VIDEO_EXTENSIONS


def stem(path):
    return os.path.splitext(os.path.basename(path))[0]


def list_files(directory, relative_to):
    paths = []
    for root, dirs, files in os.walk(directory):
        dirs.sort()
        for name in sorted(files):
            if not is_video(name):
                continue
            paths.append(os.path.relpath(os.path.join(root, name), relative_to))
    return paths


def char_vectors(texts):
    return normalize(
        TfidfVectorizer(analyzer="char_wb", ngram_range=(3, 5)).fit_transform(texts).toarray()
    )


def agglomerate(vectors):
    if len(vectors) < 2:
        return np.zeros(len(vectors), dtype=int)
    return AgglomerativeClustering(
        n_clusters=None,
        distance_threshold=DISTANCE_THRESHOLD,
        metric="cosine",
        linkage="average",
    ).fit_predict(vectors)


def cluster_paths(paths, parents):
    names = [os.path.basename(path) for path in paths]
    parent_vectors = char_vectors(parents)
    name_vectors = char_vectors(names)
    vectors = np.hstack([parent_vectors * PARENT_WEIGHT, name_vectors])
    return agglomerate(vectors)


def cluster_filenames(paths):
    return agglomerate(char_vectors([stem(path) for path in paths]))


def named_clusters(paths, labels, parent_name):
    groups = defaultdict(list)
    for path, label in zip(paths, labels):
        groups[label].append(path)
    ordered = sorted(groups, key=lambda label: len(groups[label]), reverse=True)
    named = []
    for index, label in enumerate(ordered):
        members = groups[label]
        name = parent_name if index == 0 else stem(members[0])
        named.append((name, members))
    return named


def loose_against_seasons(loose, season_paths):
    vectorizer = TfidfVectorizer(analyzer="char_wb", ngram_range=(3, 5))
    season_vectors = normalize(
        vectorizer.fit_transform([stem(path) for path in season_paths]).toarray()
    )
    loose_vectors = normalize(vectorizer.transform([stem(path) for path in loose]).toarray())
    centroid = season_vectors.mean(axis=0, keepdims=True)
    labels = cluster_filenames(loose)
    groups = defaultdict(list)
    indexes = defaultdict(list)
    for index, (path, label) in enumerate(zip(loose, labels)):
        groups[label].append(path)
        indexes[label].append(index)
    distances = {
        label: cosine_distances(loose_vectors[idxs].mean(axis=0, keepdims=True), centroid)[0, 0]
        for label, idxs in indexes.items()
    }
    farthest = max(distances.values())
    extras = []
    movies = []
    for label, members in groups.items():
        if math.isclose(distances[label], farthest):
            movies.append(members)
        else:
            extras.extend(members)
    return extras, movies


def group_library(data_dir):
    show = os.path.basename(data_dir)
    season_dirs = []
    title_dirs = []
    loose = []
    for name in sorted(os.listdir(data_dir)):
        path = os.path.join(data_dir, name)
        if os.path.isdir(path):
            if SEASON_RE.match(name):
                season_dirs.append(name)
            else:
                title_dirs.append(name)
        elif os.path.isfile(path) and is_video(name):
            loose.append(name)

    titles = []
    for folder in title_dirs:
        paths = list_files(os.path.join(data_dir, folder), data_dir)
        if not paths:
            continue
        labels = cluster_paths(paths, [folder] * len(paths))
        titles.extend(named_clusters(paths, labels, folder))

    season_paths = []
    for folder in season_dirs:
        season_paths.extend(list_files(os.path.join(data_dir, folder), data_dir))
    if season_paths:
        cluster_paths(season_paths, [show] * len(season_paths))
        show_files = list(season_paths)
        if loose:
            extras, movies = loose_against_seasons(loose, season_paths)
            show_files.extend(extras)
            titles.append((show, show_files))
            for movie in movies:
                titles.append((stem(movie[0]), movie))
        else:
            titles.append((show, show_files))
    else:
        for name in loose:
            titles.append((stem(name), [name]))
    return titles


def main():
    for subdir in sorted(os.listdir(ROOT_DIR)):
        print(f"{subdir}:")
        for name, members in group_library(os.path.join(ROOT_DIR, subdir)):
            print(f"\t{name} ({len(members)})")
        print(80 * "-")


if __name__ == "__main__":
    main()
