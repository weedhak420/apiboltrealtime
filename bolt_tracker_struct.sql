-- phpMyAdmin SQL Dump
-- version 5.2.1
-- https://www.phpmyadmin.net/
--
-- Host: 127.0.0.1
-- Generation Time: Oct 08, 2025 at 04:16 PM
-- Server version: 10.4.32-MariaDB
-- PHP Version: 8.2.12

SET SQL_MODE = "NO_AUTO_VALUE_ON_ZERO";
START TRANSACTION;
SET time_zone = "+00:00";


/*!40101 SET @OLD_CHARACTER_SET_CLIENT=@@CHARACTER_SET_CLIENT */;
/*!40101 SET @OLD_CHARACTER_SET_RESULTS=@@CHARACTER_SET_RESULTS */;
/*!40101 SET @OLD_COLLATION_CONNECTION=@@COLLATION_CONNECTION */;
/*!40101 SET NAMES utf8mb4 */;

--
-- Database: `bolt_tracker`
--

-- --------------------------------------------------------

--
-- Table structure for table `api_logs`
--

CREATE TABLE `api_logs` (
  `id` int(11) NOT NULL,
  `location_name` varchar(255) NOT NULL,
  `status_code` int(11) DEFAULT NULL,
  `response_time_ms` int(11) DEFAULT NULL,
  `success` tinyint(1) DEFAULT 0,
  `error_message` text DEFAULT NULL,
  `recorded_at` datetime DEFAULT current_timestamp()
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- --------------------------------------------------------

--
-- Table structure for table `location_cache`
--

CREATE TABLE `location_cache` (
  `location_name` varchar(255) NOT NULL,
  `lat` decimal(10,8) NOT NULL,
  `lng` decimal(11,8) NOT NULL,
  `vehicle_count` int(11) DEFAULT 0,
  `last_updated` datetime DEFAULT current_timestamp(),
  `success` tinyint(1) DEFAULT 0,
  `error` text DEFAULT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- --------------------------------------------------------

--
-- Table structure for table `performance_stats`
--

CREATE TABLE `performance_stats` (
  `id` int(11) NOT NULL,
  `metric_name` varchar(100) NOT NULL,
  `metric_value` decimal(10,4) NOT NULL,
  `recorded_at` datetime DEFAULT current_timestamp()
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- --------------------------------------------------------

--
-- Table structure for table `vehicle_cache`
--

CREATE TABLE `vehicle_cache` (
  `id` varchar(255) NOT NULL,
  `lat` decimal(10,8) NOT NULL,
  `lng` decimal(11,8) NOT NULL,
  `bearing` decimal(5,2) DEFAULT 0.00,
  `icon_url` text DEFAULT NULL,
  `category_name` varchar(255) DEFAULT NULL,
  `category_id` varchar(255) DEFAULT NULL,
  `source_location` varchar(255) DEFAULT NULL,
  `timestamp` datetime DEFAULT NULL,
  `distance` decimal(10,2) DEFAULT 0.00,
  `created_at` datetime DEFAULT current_timestamp()
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- --------------------------------------------------------

--
-- Table structure for table `vehicle_history`
--

CREATE TABLE `vehicle_history` (
  `history_id` bigint(20) NOT NULL,
  `vehicle_id` varchar(255) DEFAULT NULL,
  `lat` double DEFAULT NULL,
  `lng` double DEFAULT NULL,
  `bearing` int(11) DEFAULT NULL,
  `category_name` varchar(255) DEFAULT NULL,
  `timestamp` datetime DEFAULT NULL,
  `created_at` timestamp NOT NULL DEFAULT current_timestamp(),
  `speed` decimal(5,2) DEFAULT 0.00,
  `source_location` varchar(255) DEFAULT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

--
-- Indexes for dumped tables
--

--
-- Indexes for table `api_logs`
--
ALTER TABLE `api_logs`
  ADD PRIMARY KEY (`id`),
  ADD KEY `idx_location` (`location_name`),
  ADD KEY `idx_recorded_at` (`recorded_at`),
  ADD KEY `idx_success` (`success`);

--
-- Indexes for table `location_cache`
--
ALTER TABLE `location_cache`
  ADD PRIMARY KEY (`location_name`),
  ADD KEY `idx_location_updated` (`last_updated`),
  ADD KEY `idx_location_success` (`success`);

--
-- Indexes for table `performance_stats`
--
ALTER TABLE `performance_stats`
  ADD PRIMARY KEY (`id`),
  ADD KEY `idx_metric_name` (`metric_name`),
  ADD KEY `idx_recorded_at` (`recorded_at`);

--
-- Indexes for table `vehicle_cache`
--
ALTER TABLE `vehicle_cache`
  ADD PRIMARY KEY (`id`),
  ADD KEY `idx_vehicle_timestamp` (`timestamp`),
  ADD KEY `idx_vehicle_location` (`source_location`),
  ADD KEY `idx_vehicle_created` (`created_at`),
  ADD KEY `idx_vehicle_category` (`category_name`),
  ADD KEY `idx_vehicle_created_at` (`created_at`);

--
-- Indexes for table `vehicle_history`
--
ALTER TABLE `vehicle_history`
  ADD PRIMARY KEY (`history_id`),
  ADD KEY `idx_vehicle_id` (`vehicle_id`),
  ADD KEY `idx_timestamp` (`timestamp`),
  ADD KEY `idx_created_at` (`created_at`),
  ADD KEY `idx_vh_time` (`timestamp`),
  ADD KEY `idx_vh_vehicle` (`vehicle_id`,`timestamp`),
  ADD KEY `idx_vh_cat_time` (`category_name`,`timestamp`),
  ADD KEY `idx_vehicle_history_time` (`timestamp`),
  ADD KEY `idx_vehicle_history_vehicle` (`vehicle_id`),
  ADD KEY `idx_vehicle_history_latlng` (`lat`,`lng`),
  ADD KEY `idx_vehicle_history_vehicle_time` (`vehicle_id`,`timestamp`),
  ADD KEY `idx_vehicle_history_time_latlng` (`timestamp`,`lat`,`lng`),
  ADD KEY `idx_vehicle_history_category` (`category_name`),
  ADD KEY `idx_vehicle_history_created_at` (`created_at`),
  ADD KEY `idx_vehicle_history_bearing` (`bearing`),
  ADD KEY `idx_vehicle_history_analytics` (`timestamp`,`lat`,`lng`,`category_name`),
  ADD KEY `idx_vehicle_history_timestamp` (`timestamp`),
  ADD KEY `idx_vehicle_history_vehicle_id` (`vehicle_id`),
  ADD KEY `idx_vehicle_history_vehicle_timestamp` (`vehicle_id`,`timestamp`),
  ADD KEY `idx_vehicle_history_timestamp_desc` (`timestamp`),
  ADD KEY `idx_vehicle_history_speed` (`speed`),
  ADD KEY `idx_vehicle_history_source` (`source_location`),
  ADD KEY `idx_vehicle_history_heatmap` (`timestamp`,`lat`,`lng`,`vehicle_id`),
  ADD KEY `idx_vehicle_history_timestamp_hour` (`timestamp`),
  ADD KEY `idx_vehicle_history_timestamp_day` (`timestamp`),
  ADD KEY `idx_vehicle_history_lat` (`lat`),
  ADD KEY `idx_vehicle_history_lng` (`lng`),
  ADD KEY `idx_vehicle_history_lat_lng` (`lat`,`lng`);

--
-- AUTO_INCREMENT for dumped tables
--

--
-- AUTO_INCREMENT for table `api_logs`
--
ALTER TABLE `api_logs`
  MODIFY `id` int(11) NOT NULL AUTO_INCREMENT;

--
-- AUTO_INCREMENT for table `performance_stats`
--
ALTER TABLE `performance_stats`
  MODIFY `id` int(11) NOT NULL AUTO_INCREMENT;

--
-- AUTO_INCREMENT for table `vehicle_history`
--
ALTER TABLE `vehicle_history`
  MODIFY `history_id` bigint(20) NOT NULL AUTO_INCREMENT;
COMMIT;

/*!40101 SET CHARACTER_SET_CLIENT=@OLD_CHARACTER_SET_CLIENT */;
/*!40101 SET CHARACTER_SET_RESULTS=@OLD_CHARACTER_SET_RESULTS */;
/*!40101 SET COLLATION_CONNECTION=@OLD_COLLATION_CONNECTION */;
